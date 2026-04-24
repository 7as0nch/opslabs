/* *
 * @Author: chengjiang
 * @Date: 2026-04-21 18:16:37
 * @Description: GCServer —— opslabs 场景资源周期回收器
 *               README Day 6 规划的 task.GCServer 统一实现位置
 *               实现 kratos.transport.Server,和 http/grpc 一同注册到 kratos.App
 *
 * 职责(当前 V1):
 *   1. idle 回收:running attempt 的 LastActiveAt 超过 idleCutoff → Terminate
 *      (docker Stop + 释放端口 + store.Delete + DB status=terminated)
 *   2. passed 复盘窗口过期:passed attempt 的 FinishedAt 超过 passedGrace → CleanupPassed
 *      (只停容器,不改 status,保留通关记录供复盘)
 *   3. stale-container 扫描(Round 5 新增):对 running sandbox attempt 周期 Ping
 *      容器死了就清 store + DB 标 terminated,避免"store 说 running 但 iframe 连不上"的
 *      僵尸记录(用户场景:手动 docker rm / Docker Desktop 重启)
 *
 * 未来扩展口子(按优先级):
 *   - per-user 并发上限检查(同一 clientID 同时超过 3 个 running 触发强制清理)
 *   - 端口池健康度上报(阈值告警)
 *   - 孤儿容器扫描(docker ps 出来但 store / DB 不认识的 opslabs.* 容器)
**/
package server

import (
	"context"
	"errors"
	"time"

	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/runtime"
	"github.com/7as0nch/backend/internal/store"
	"go.uber.org/zap"
)

// DefaultPassedGrace 通关后给用户复盘的宽限时间,到期容器被 GCServer 清理
//
// 为什么 30min:用户期望"通关后容器保留一段时间用来复盘",30min 和 idle 一致,
// 不必额外一套定时器。场景级可通过 passed_grace_minutes 覆盖。
const DefaultPassedGrace = 30 * time.Minute

// GCServer 周期性扫 AttemptStore,回收 idle / passed-grace-over / stale 容器
//
// 行为细节见文件头 Description。所有分支失败都只记日志不 panic,因为 GC 是后台
// 周期任务,一次失败 1min 后会再试,不能因为单次异常把 kratos app 打挂。
//
// Round 6 起:
//   - AttemptBootstrapper 删除,启动期 runner.Reconcile 合并到 Start 里
//   - AttemptReaper / NewAttemptReaper 旧名字一并清掉(在 GCServer 之后没有调用方)
type GCServer struct {
	store  *store.AttemptStore
	uc     *attempt.AttemptUsecase
	runner runtime.Runner
	log    *zap.Logger

	interval    time.Duration
	idleCutoff  time.Duration
	passedGrace time.Duration

	cancel context.CancelFunc
	doneCh chan struct{}
}

// NewGCServer 构造(时间参数从 conf.Runtime 读)
//
// 参数:
//   - s        : 共享的 AttemptStore
//   - uc       : AttemptUsecase(用它的 Terminate / CleanupPassed 才能统一走过状态机)
//   - runner   : 运行时接口,V1 用来做 stale-container Ping;可为 nil(mock 场景)
//   - c        : 读 reaper_interval / default_idle_timeout
//   - logger   : 日志
func NewGCServer(
	s *store.AttemptStore,
	uc *attempt.AttemptUsecase,
	runner runtime.Runner,
	c *conf.Bootstrap,
	logger *zap.Logger,
) *GCServer {
	interval := time.Minute
	idle := 30 * time.Minute
	grace := DefaultPassedGrace
	if c != nil && c.Runtime != nil {
		if c.Runtime.ReaperInterval != nil {
			if d := c.Runtime.ReaperInterval.AsDuration(); d > 0 {
				interval = d
			}
		}
		if c.Runtime.DefaultIdleTimeout != nil {
			if d := c.Runtime.DefaultIdleTimeout.AsDuration(); d > 0 {
				idle = d
			}
		}
		// passed_grace 字段未来加入 conf.Runtime 后在这里读
	}
	return &GCServer{
		store:       s,
		uc:          uc,
		runner:      runner,
		log:         logger,
		interval:    interval,
		idleCutoff:  idle,
		passedGrace: grace,
		doneCh:      make(chan struct{}),
	}
}

// Start 实现 kratos.transport.Server.Start
//
// Round 6 之前这里只起一个 ticker goroutine;attempt_bootstrap 废除后,
// 进程启动时的一次性 runner.Reconcile(清理上次残留 opslabs.* 容器)也并到这里:
//   - 同步执行:必须在 ticker 第一轮跑之前完成,否则 tick 可能碰到
//     "Redis 里没有但 docker 还有"的 zombie 容器(snapshot 不到就不会清)
//   - 失败不阻塞 Server.Start:runner 只是清理器,挂了对新 attempt 没影响,
//     只让运维从日志 Warn 定位即可
func (g *GCServer) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	g.cancel = cancel
	g.log.Info("gc server started",
		zap.Duration("interval", g.interval),
		zap.Duration("idle_cutoff", g.idleCutoff),
		zap.Duration("passed_grace", g.passedGrace))

	// 启动期残余容器清理(取代旧 AttemptBootstrapper)
	if g.runner != nil {
		if err := g.runner.Reconcile(ctx); err != nil {
			g.log.Warn("gc startup reconcile failed (old containers may leak until next process restart)",
				zap.Error(err))
		}
	}

	go func() {
		defer close(g.doneCh)
		ticker := time.NewTicker(g.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				g.log.Info("gc server exiting")
				return
			case now := <-ticker.C:
				g.tick(ctx, now)
			}
		}
	}()
	return nil
}

// Stop 实现 kratos.transport.Server.Stop —— 停 ticker 并等 goroutine 退出
func (g *GCServer) Stop(ctx context.Context) error {
	if g.cancel != nil {
		g.cancel()
	}
	select {
	case <-g.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// tick 单次扫描:idle + passed-grace + stale-container 三路并行判定
//
// 顺序:
//   1. stale-container Ping:最快排除"容器已死但 store 还记着"的僵尸
//   2. idle 回收:给 Ping 过的 alive 容器做 LastActiveAt 判定
//   3. passed grace:和前两条解耦,独立分支
func (g *GCServer) tick(ctx context.Context, now time.Time) {
	list, err := g.store.Snapshot(ctx)
	if err != nil {
		// Redis 抖动:本轮跳过,1min 后再试;别把 tick goroutine 打挂
		g.log.Warn("gc tick snapshot failed, skip this round", zap.Error(err))
		return
	}
	for _, a := range list {
		switch a.Status {
		case "running":
			// 先探活:如果容器已经不在(docker rm / 宿主重启),直接清 store + mark DB terminated
			// 不走 uc.Terminate,避免"runner.Stop 一个不存在的容器"浪费 exec docker CLI
			if g.runner != nil && a.HostPort > 0 && a.ContainerID != "" {
				if err := g.runner.Ping(ctx, a.ContainerID); err != nil {
					g.log.Info("gc stale container detected",
						zap.Int64("id", a.ID),
						zap.String("slug", a.ScenarioSlug),
						zap.String("container_id", a.ContainerID),
						zap.Error(err))
					g.cleanupStale(ctx, a.ID)
					continue
				}
			}
			// Ping 通过 / 非 sandbox:走 idle 判定
			idle := now.Sub(a.LastActiveAt)
			if idle < g.idleCutoff {
				continue
			}
			g.log.Info("gc reaping idle attempt",
				zap.Int64("id", a.ID),
				zap.String("slug", a.ScenarioSlug),
				zap.Duration("idle", idle))
			if err := g.uc.Terminate(ctx, a.ID); err != nil && !errors.Is(err, context.Canceled) {
				g.log.Error("gc terminate failed",
					zap.Int64("id", a.ID),
					zap.Error(err))
			}
		case "passed":
			// passed 走独立 grace 窗口,不受 stale Ping 影响(通关后容器还活着是正常的)
			if a.FinishedAt == nil {
				// 异常:passed 但没 FinishedAt,用 LastActiveAt 兜底
				if now.Sub(a.LastActiveAt) < g.passedGrace {
					continue
				}
			} else if now.Sub(*a.FinishedAt) < g.passedGrace {
				continue
			}
			g.log.Info("gc reaping passed-grace attempt",
				zap.Int64("id", a.ID),
				zap.String("slug", a.ScenarioSlug))
			if err := g.uc.CleanupPassed(ctx, a.ID); err != nil && !errors.Is(err, context.Canceled) {
				g.log.Error("gc cleanup passed failed",
					zap.Int64("id", a.ID),
					zap.Error(err))
			}
		}
	}
}

// cleanupStale stale-container 场景:容器已死,只做"清 store + mark DB terminated"
// 不调 runner.Stop —— 容器已经不存在了,再 Stop 会打日志污染
//
// 失败只记日志不阻塞:下一轮 tick 会重试,最坏情况 DB 里僵尸记录多存 1 个 interval
func (g *GCServer) cleanupStale(ctx context.Context, id int64) {
	// 从 Redis 拿完整快照来构造 MarkTerminated 参数
	a, ok, err := g.store.Get(ctx, id)
	if err != nil {
		g.log.Warn("gc cleanup stale: store Get failed (will retry next tick)",
			zap.Int64("id", id), zap.Error(err))
		return
	}
	if !ok {
		// 已被别的路径清了,no-op
		return
	}
	if err := g.store.Delete(ctx, id); err != nil {
		g.log.Warn("gc cleanup stale: store Delete failed (will retry next tick)",
			zap.Int64("id", id), zap.Error(err))
		return
	}
	a.MarkTerminated(time.Now())
	// 独立 ctx 2s 超时,避免 DB 抖动拖慢整个 tick 循环
	deadCtx, deadCancel := context.WithTimeout(ctx, 2*time.Second)
	defer deadCancel()
	if err := g.uc.MarkTerminatedInDB(deadCtx, a); err != nil {
		g.log.Warn("gc cleanup stale DB update failed (will retry next tick)",
			zap.Int64("id", id),
			zap.Error(err))
	}
}

// 旧 AttemptReaper / NewAttemptReaper 兼容别名已于 Round 6 移除。
// 所有调用方请直接使用 GCServer / NewGCServer。
