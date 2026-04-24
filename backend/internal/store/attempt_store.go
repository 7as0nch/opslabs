/* *
 * @Author: chengjiang
 * @Date: 2026-04-22
 * @Description: Attempt 共享缓存,Redis 版
 *
 *               Round 6 从"进程内 map + sync.RWMutex"迁到 Redis,原因:
 *                 1. 多实例部署能共享 attempt 状态
 *                 2. 进程重启不丢活跃 attempt(不再需要 bootstrap 回灌)
 *                 3. TTL 兜底"永远不会忘记清理"的进程内 map 死角
 *
 *               数据布局见 consts/redis.go 的 RedisKey* 注释:
 *                 - opslabs:attempt:<id>                (主 key, JSON)
 *                 - opslabs:attempt:active              (SET of id 字符串)
 *                 - opslabs:attempt:owner:client:<c>:<s>
 *                 - opslabs:attempt:owner:user:<u>:<s>
 *
 *               API 语义:
 *                 - 所有方法都吃 ctx,网络失败返 error(调用方自己决定降级 / 传上去)
 *                 - Get / FindActive* 三元组返回:(*Attempt, bool, error)
 *                     - 命中      -> (a, true, nil)
 *                     - 未命中    -> (nil, false, nil)
 *                     - 网络错   -> (nil, false, err)
 *                 - Put / Delete / Update* 失败直接返 error
**/
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/7as0nch/backend/internal/consts"
	"github.com/7as0nch/backend/internal/db"
	"github.com/7as0nch/backend/models/generator/model"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DefaultTTL attempt 主 key + owner 索引的默认 TTL
//
// 选 60min 的理由:
//   - GCServer idleCutoff 默认 30min,TTL 给双倍冗余,避免 GC 抖动导致误删
//   - running attempt 每次 Put / UpdateLastActive 都会 Expire 刷新,不会"做着做着突然过期"
//   - passed 进入复盘后 FinishedAt+30min grace,60min 够用户看完结果
//
// 不够用时通过 NewAttemptStore 的 opts 覆写
const DefaultTTL = 60 * time.Minute

// AttemptStore Redis-backed 共享缓存
//
// 和旧内存版 AttemptStore 的 API 差异:
//   - 所有方法第一个参数是 ctx
//   - 读类方法返回 (value, ok, err) 三元组而不是 (value, ok)
//   - 写类方法返回 error
//
// 并发安全:所有操作委托给 Redis 服务端,客户端无需本地锁
type AttemptStore struct {
	redis db.RedisRepo
	log   *zap.Logger
	ttl   time.Duration
}

// Option 可选配置(目前只有 TTL,保留扩展位)
type Option func(*AttemptStore)

// WithTTL 覆盖默认 TTL(主 key + owner 索引)
//
// ttl <= 0 时回退到 DefaultTTL,避免"立即过期"的误用
func WithTTL(ttl time.Duration) Option {
	return func(s *AttemptStore) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

// NewAttemptStore 构造 Redis-backed store
//
// Round 6 起 redis 是强依赖,调用方 (server.NewAttemptStore) 保证非 nil;
// 这里依然对 nil 做防御,log.Fatal 让问题在启动阶段立刻暴露而不是运行时崩
func NewAttemptStore(redis db.RedisRepo, logger *zap.Logger, opts ...Option) *AttemptStore {
	if redis == nil {
		logger.Fatal("attempt store requires redis, but got nil — check config.data.redis")
	}
	s := &AttemptStore{
		redis: redis,
		log:   logger,
		ttl:   DefaultTTL,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ------------------------------------------------------------
// 内部 key helpers —— 集中在一处方便 rename/调试
// ------------------------------------------------------------

func keyAttempt(id int64) string {
	return fmt.Sprintf(consts.RedisKeyAttempt, id)
}

func keyOwnerClient(clientID, slug string) string {
	return fmt.Sprintf(consts.RedisKeyAttemptOwnerClient, clientID, slug)
}

func keyOwnerUser(userID int64, slug string) string {
	return fmt.Sprintf(consts.RedisKeyAttemptOwnerUser, userID, slug)
}

// ------------------------------------------------------------
// Put
// ------------------------------------------------------------

// Put 写入 / 覆盖 attempt,同时维护 active SET 和 owner 索引
//
// 一次 pipeline 打包多命令:
//   1. SET 主 key(JSON)TTL
//   2. SADD active SET
//   3. SET owner:client 索引(如果 ClientID 非空)+ TTL
//   4. SET owner:user 索引(如果 UserID > 0)  + TTL
//
// active SET 本身无 TTL,依赖 Delete / UpdateStatus(finished) 时显式 SREM 清理,
// 避免"主 key 过期了但 SET 里还挂着一个死 id"的现象 —— Snapshot 对此容忍:
// MGet 返回 nil 的位置会被跳过
func (s *AttemptStore) Put(ctx context.Context, a *model.OpslabsAttempt) error {
	if a == nil {
		return errors.New("attempt store: Put nil attempt")
	}
	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("attempt store: marshal: %w", err)
	}
	mainKey := keyAttempt(a.ID)

	pipe := s.redis.Pipeline()
	pipe.Set(ctx, mainKey, data, s.ttl)
	pipe.SAdd(ctx, consts.RedisKeyAttemptActive, strconv.FormatInt(a.ID, 10))
	if a.ClientID != "" {
		k := keyOwnerClient(a.ClientID, a.ScenarioSlug)
		pipe.Set(ctx, k, strconv.FormatInt(a.ID, 10), s.ttl)
	}
	if a.UserID > 0 {
		k := keyOwnerUser(a.UserID, a.ScenarioSlug)
		pipe.Set(ctx, k, strconv.FormatInt(a.ID, 10), s.ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		s.log.Error("attempt store Put pipeline failed",
			zap.Int64("id", a.ID),
			zap.String("slug", a.ScenarioSlug),
			zap.Error(err))
		return fmt.Errorf("attempt store: put: %w", err)
	}
	return nil
}

// ------------------------------------------------------------
// Get
// ------------------------------------------------------------

// Get 按 ID 查主 key
//
// 返回:
//   - 命中 -> (a, true, nil)
//   - 未命中 / 已过期 -> (nil, false, nil)
//   - Redis 错误 -> (nil, false, err)
func (s *AttemptStore) Get(ctx context.Context, id int64) (*model.OpslabsAttempt, bool, error) {
	raw, err := s.redis.Get(ctx, keyAttempt(id))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("attempt store: get: %w", err)
	}
	var a model.OpslabsAttempt
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		// JSON 损坏就当没命中,同时把脏数据删掉避免下次继续炸
		s.log.Warn("attempt store Get: corrupted json, dropping", zap.Int64("id", id), zap.Error(err))
		_ = s.redis.Del(ctx, keyAttempt(id))
		_ = s.redis.SRem(ctx, consts.RedisKeyAttemptActive, strconv.FormatInt(id, 10))
		return nil, false, nil
	}
	return &a, true, nil
}

// ------------------------------------------------------------
// Delete
// ------------------------------------------------------------

// Delete 彻底清掉 attempt 的所有索引
//
// 幂等:即使 key 都不存在也不会报错
// 清理范围:主 key + active SET 成员 + 两条 owner 索引(先读主 key 拿 clientID/userID)
func (s *AttemptStore) Delete(ctx context.Context, id int64) error {
	// 先读一次主 key 拿 owner 信息,否则没法定位 owner 索引
	// 读不到也没关系 —— 可能已经被 TTL 过期了,active SET 里还挂着就只删 SET 成员
	a, ok, err := s.Get(ctx, id)
	if err != nil {
		// Get 已经降级处理过(JSON 损坏),这里只管网络错
		return err
	}

	pipe := s.redis.Pipeline()
	pipe.Del(ctx, keyAttempt(id))
	pipe.SRem(ctx, consts.RedisKeyAttemptActive, strconv.FormatInt(id, 10))
	if ok && a != nil {
		if a.ClientID != "" {
			pipe.Del(ctx, keyOwnerClient(a.ClientID, a.ScenarioSlug))
		}
		if a.UserID > 0 {
			pipe.Del(ctx, keyOwnerUser(a.UserID, a.ScenarioSlug))
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("attempt store: delete: %w", err)
	}
	return nil
}

// ------------------------------------------------------------
// UpdateLastActive
// ------------------------------------------------------------

// UpdateLastActive 刷新 LastActiveAt + 延长 TTL
//
// 读-改-写模式:先 Get 拿完整快照,改 LastActiveAt,再 Put 回去。
// Put 内部重跑 pipeline,顺便把 owner 索引的 TTL 一起续上。
//
// 返回 (ok, err):
//   - key 不存在 -> (false, nil)
//   - 更新成功    -> (true, nil)
//   - 网络错      -> (false, err)
func (s *AttemptStore) UpdateLastActive(ctx context.Context, id int64, now time.Time) (bool, error) {
	a, ok, err := s.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	a.LastActiveAt = now
	if err := s.Put(ctx, a); err != nil {
		return false, err
	}
	return true, nil
}

// ------------------------------------------------------------
// UpdateStatus
// ------------------------------------------------------------

// UpdateStatus 把 biz 层改过状态的 Attempt 同步回缓存
//
// 和 Put 的区别在于:status 进入终态(passed/terminated/expired/failed 之外)
// 时,调用方应该直接 Delete 而不是 UpdateStatus;UpdateStatus 仅负责"仍然活着
// 但字段变了"的场景(比如 MarkPassed 后 passed 阶段还在复盘窗口内算活)
//
// key 不存在时返回 (false, nil),避免"先过期后 UpdateStatus"的误报
func (s *AttemptStore) UpdateStatus(ctx context.Context, a *model.OpslabsAttempt) (bool, error) {
	if a == nil {
		return false, errors.New("attempt store: UpdateStatus nil attempt")
	}
	// 存在性用 EXISTS 判更省流量,但 pipeline 多一次 RTT 不如直接 Get + 条件 Put
	_, ok, err := s.Get(ctx, a.ID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if err := s.Put(ctx, a); err != nil {
		return false, err
	}
	return true, nil
}

// ------------------------------------------------------------
// Snapshot
// ------------------------------------------------------------

// Snapshot 返回所有活跃 attempt 的快照,GCServer.tick 用
//
// 流程:SMEMBERS active -> MGet 主 keys -> 反序列化
// 容错:
//   - active SET 里挂着但主 key 已过期 -> 跳过,不出现在返回里
//   - MGet 某位置 JSON 损坏 -> 记日志并跳过(不中断整个 tick)
//   - active SET 为空 -> 返回空切片,不访问 MGet(MGet keys=0 会报错)
func (s *AttemptStore) Snapshot(ctx context.Context) ([]*model.OpslabsAttempt, error) {
	ids, err := s.redis.SMembers(ctx, consts.RedisKeyAttemptActive)
	if err != nil {
		return nil, fmt.Errorf("attempt store: snapshot SMembers: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(ids))
	for _, idStr := range ids {
		id, perr := strconv.ParseInt(idStr, 10, 64)
		if perr != nil {
			s.log.Warn("attempt store Snapshot: bad id in active set", zap.String("raw", idStr))
			continue
		}
		keys = append(keys, keyAttempt(id))
	}
	vals, err := s.redis.MGet(ctx, keys...)
	if err != nil {
		return nil, fmt.Errorf("attempt store: snapshot MGet: %w", err)
	}
	out := make([]*model.OpslabsAttempt, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			// 主 key 已过期,active SET 里还挂着死 id —— 顺手清掉避免下次再扫
			if i < len(ids) {
				_ = s.redis.SRem(ctx, consts.RedisKeyAttemptActive, ids[i])
			}
			continue
		}
		str, ok := v.(string)
		if !ok {
			s.log.Warn("attempt store Snapshot: unexpected value type", zap.String("key", keys[i]))
			continue
		}
		var a model.OpslabsAttempt
		if err := json.Unmarshal([]byte(str), &a); err != nil {
			s.log.Warn("attempt store Snapshot: bad json, dropping",
				zap.String("key", keys[i]),
				zap.Error(err))
			_ = s.redis.Del(ctx, keys[i])
			if i < len(ids) {
				_ = s.redis.SRem(ctx, consts.RedisKeyAttemptActive, ids[i])
			}
			continue
		}
		out = append(out, &a)
	}
	return out, nil
}

// ------------------------------------------------------------
// FindActiveByClientSlug / FindActiveByUserSlug
// ------------------------------------------------------------

// FindActiveByClientSlug 按 (clientID, slug) 反查活跃 attempt
//
// 走 owner 索引一步到位:GET owner 拿 id -> GET 主 key 拿数据
// 索引命中但主 key 失效时视为未命中(主 key TTL 到了就当 attempt 已过期)
//
// clientID 空串直接返 (nil, false, nil) —— 防止退化到 UserID=0 的全局共享命名空间
func (s *AttemptStore) FindActiveByClientSlug(ctx context.Context, clientID, slug string) (*model.OpslabsAttempt, bool, error) {
	if clientID == "" {
		return nil, false, nil
	}
	return s.findActiveByOwnerKey(ctx, keyOwnerClient(clientID, slug))
}

// FindActiveByUserSlug 同上,维度换成 UserID
//
// Deprecated: V1 主用 FindActiveByClientSlug;等登录接入后本方法会切回主位
func (s *AttemptStore) FindActiveByUserSlug(ctx context.Context, userID int64, slug string) (*model.OpslabsAttempt, bool, error) {
	if userID <= 0 {
		return nil, false, nil
	}
	return s.findActiveByOwnerKey(ctx, keyOwnerUser(userID, slug))
}

// findActiveByOwnerKey FindActiveBy* 的公共实现
//
// 语义:
//   - owner 索引未命中 -> (nil, false, nil)
//   - owner 索引命中但主 key 过期 / 数据损坏 -> (nil, false, nil),顺手清掉死索引
//   - 主 key status 不是 running/passed(已结束但索引没来得及清)-> (nil, false, nil)
func (s *AttemptStore) findActiveByOwnerKey(ctx context.Context, ownerKey string) (*model.OpslabsAttempt, bool, error) {
	raw, err := s.redis.Get(ctx, ownerKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("attempt store: find by owner: %w", err)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		s.log.Warn("attempt store FindActive: bad id in owner index",
			zap.String("owner_key", ownerKey),
			zap.String("raw", raw))
		_ = s.redis.Del(ctx, ownerKey)
		return nil, false, nil
	}
	a, ok, err := s.Get(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		// 索引命中但主 key 失效,清索引避免下次再跳
		_ = s.redis.Del(ctx, ownerKey)
		return nil, false, nil
	}
	if a.Status != model.AttemptStatusRunning && a.Status != model.AttemptStatusPassed {
		// 已结束的 attempt 不应该还挂着 owner 索引,但真挂了就顺手清
		_ = s.redis.Del(ctx, ownerKey)
		return nil, false, nil
	}
	return a, true, nil
}

// ------------------------------------------------------------
// Len
// ------------------------------------------------------------

// Len 活跃 attempt 数量,走 SCARD 一次 RTT
//
// 注:SCARD 返回的是 SET 基数,可能包含"主 key 已过期但 SET 成员没来得及清"的脏数据,
// 数值用于监控 / 调试可接受小幅偏差;严格计数请走 Snapshot 过滤 nil 再算
func (s *AttemptStore) Len(ctx context.Context) (int, error) {
	ids, err := s.redis.SMembers(ctx, consts.RedisKeyAttemptActive)
	if err != nil {
		return 0, fmt.Errorf("attempt store: len: %w", err)
	}
	return len(ids), nil
}
