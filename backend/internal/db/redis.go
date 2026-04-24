/* *
 * @Author: chengjiang
 * @Date: 2026-03-16 16:48:00
 * @Description:
**/
package db

import (
	"context"
	"time"

	"github.com/7as0nch/backend/internal/conf"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, key string) error
	GetDel(ctx context.Context, key string) (string, error)
	RPush(ctx context.Context, key string, expiration time.Duration, values ...interface{}) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	// ------------------------------------------------------------
	// Round 6 opslabs AttemptStore 迁 Redis 新增的原语
	// ------------------------------------------------------------
	// SAdd / SRem / SMembers: SET 操作,active attemptID 集合用
	// MGet: 批量取主 key(SMEMBERS → MGET 的读热路径)
	// Expire: 单独刷 TTL(Put 后延长主 key + owner 索引的过期时间)
	// Pipeline: 主 key + SET + owner 索引的原子批量写,单次 RTT
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SRem(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Pipeline() redis.Pipeliner
	Close() error
}

type redisRepo struct {
	client *redis.Client
}

func NewRedis(c *conf.Bootstrap, logger *zap.Logger) (RedisRepo, error) {
	if c == nil || c.GetData() == nil || c.GetData().GetRedis() == nil {
		return nil, nil
	}

	rc := c.GetData().GetRedis()
	readTimeout := time.Duration(0)
	writeTimeout := time.Duration(0)
	if rc.GetReadTimeout() != nil {
		readTimeout = rc.GetReadTimeout().AsDuration()
	}
	if rc.GetWriteTimeout() != nil {
		writeTimeout = rc.GetWriteTimeout().AsDuration()
	}
	client := redis.NewClient(&redis.Options{
		Addr:         rc.GetAddr(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Password:     rc.GetPassword(),
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		logger.Error("redis ping failed", zap.Error(err))
		return nil, err
	}
	return &redisRepo{client: client}, nil
}

func (r *redisRepo) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *redisRepo) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *redisRepo) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *redisRepo) GetDel(ctx context.Context, key string) (string, error) {
	return r.client.GetDel(ctx, key).Result()
}

func (r *redisRepo) RPush(ctx context.Context, key string, expiration time.Duration, values ...interface{}) error {
	pipe := r.client.TxPipeline()
	pipe.RPush(ctx, key, values...)
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisRepo) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// SAdd 向 SET 加成员,返回值(新增几个)这里忽略
//   - 用于 opslabs:attempt:active 维护活跃 attemptID 集合
//   - 重复加同一 member 幂等,不会报错
func (r *redisRepo) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SRem 从 SET 删成员,成员不存在也不报错(幂等)
func (r *redisRepo) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, key, members...).Err()
}

// SMembers 返回 SET 所有成员
//   - 用于 Snapshot:取出所有活跃 attemptID 后再 MGet 主数据
//   - 数据量大(>千)时应改 SSCAN 避免阻塞,V1 单实例几十级不担心
func (r *redisRepo) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// MGet 批量取 key 的 String 值,返回切片长度与 keys 相同
//   - 不存在的 key 对应位置是 nil,调用方负责判空
//   - 空 keys 直接返回 ([]interface{}{}, nil),避免 redis 报错
func (r *redisRepo) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if len(keys) == 0 {
		return []interface{}{}, nil
	}
	return r.client.MGet(ctx, keys...).Result()
}

// Expire 刷新单个 key 的 TTL
//   - Put/UpdateLastActive 后延长主 key + owner 索引的过期时间
//   - expiration <= 0 时 redis 会立即删 key,调用方应保证 > 0
func (r *redisRepo) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// Pipeline 返回一个 redis.Pipeliner,用于批量打包命令
//   - 调用方 .Do/.Set/.SAdd... 后 pipeline.Exec(ctx) 一次性发送
//   - AttemptStore 的 Put / Delete 用它保证多 key 写入的批量性(非事务,仅降 RTT)
//   - 真要原子可换 TxPipeline,V1 不强求
func (r *redisRepo) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

func (r *redisRepo) Close() error {
	return r.client.Close()
}
