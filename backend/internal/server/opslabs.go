/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Opslabs 相关服务的 server 层装配:
 *                 - 场景注册表 / 内存缓存 / 运行时实例工厂
 *                 - AttemptService 的小配置(从 conf.yaml 解析)
 *                 - AttemptReaper:定时扫内存缓存,清理空闲超时的 attempt
 *               放在 server/ 和 http.go、grpc.go 同层,由 main 注册运行
**/
package server

import (
	"context"
	"errors"
	"time"

	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/runtime"
	"github.com/7as0nch/backend/internal/scenario"
	"github.com/7as0nch/backend/internal/service/opslabs"
	"github.com/7as0nch/backend/internal/store"
	"go.uber.org/zap"
)

// ==============================================================
// 场景注册表
// ==============================================================

// NewScenarioRegistry 构造硬编码场景注册表
// Week 2 换成从 scenarios/*/meta.yaml 扫描时,只改这里
func NewScenarioRegistry() scenario.Registry {
	return scenario.NewRegistry()
}

// ==============================================================
// Attempt 内存缓存
// ==============================================================

// NewAttemptStore 构造全局唯一的 Attempt 内存缓存
func NewAttemptStore() *store.AttemptStore {
	return store.NewAttemptStore()
}

// ==============================================================
// 容器运行时
// ==============================================================

// DefaultDockerNetwork docker runtime 默认加入的网络
// 提前到 scripts/dev-up.sh: docker network create opslabs-scenarios
const DefaultDockerNetwork = "opslabs-scenarios"

// NewRunner 按配置构造运行时
//
// driver 语义:
//   - docker(默认/生产):docker run ... 拉真容器,HostPort 是真实映射端口
//   - mock(测试):不启容器、不占端口,HostPort=0 → 前端知道走"预览模式"
//     线上请勿使用
//
// 为什么改默认为 docker:之前默认 mock 会给前端返回一个假端口,
// iframe 去连那个假端口时只会静默 "ERR_CONNECTION_REFUSED",用户无感知。
// 现在默认 docker,如果本机没装 Docker Desktop,显式把 yaml 改成 driver: mock 即可跑通契约。
func NewRunner(c *conf.Bootstrap, logger *zap.Logger) runtime.Runner {
	driver := "docker" // 默认改为 docker,与生产一致
	portStart := 10000
	portEnd := 19999
	dockerNetwork := DefaultDockerNetwork

	if c != nil && c.Runtime != nil {
		if c.Runtime.Driver != "" {
			driver = c.Runtime.Driver
		}
		if c.Runtime.PortStart > 0 {
			portStart = int(c.Runtime.PortStart)
		}
		if c.Runtime.PortEnd > 0 {
			portEnd = int(c.Runtime.PortEnd)
		}
	}

	switch driver {
	case "docker":
		logger.Info("runtime initialized",
			zap.String("driver", "docker"),
			zap.String("network", dockerNetwork),
			zap.Int("port_start", portStart),
			zap.Int("port_end", portEnd))
		return runtime.NewDockerRunner(portStart, portEnd, logger,
			runtime.WithDockerNetwork(dockerNetwork))
	case "mock":
		logger.Warn("runtime initialized in MOCK mode",
			zap.String("hint", "no real container, no terminal; use docker for prod"))
		return runtime.NewMockRunner()
	default:
		logger.Warn("unknown runtime driver, fallback to docker",
			zap.String("driver", driver))
		return runtime.NewDockerRunner(portStart, portEnd, logger,
			runtime.WithDockerNetwork(dockerNetwork))
	}
}

// ==============================================================
// Opslabs Service 配置(attempt service 要的终端 URL 等)
// ==============================================================

// NewOpslabsServiceOptions 从 conf.Runtime 解析出 service 层需要的小配置
func NewOpslabsServiceOptions(c *conf.Bootstrap) *opslabs.ServiceOptions {
	opts := opslabs.DefaultServiceOptions()
	if c == nil || c.Runtime == nil {
		return opts
	}
	if c.Runtime.TerminalHost != "" {
		opts.TerminalHost = c.Runtime.TerminalHost
	}
	if c.Runtime.TerminalUrlTemplate != "" {
		opts.TerminalURLTemplate = c.Runtime.TerminalUrlTemplate
	}
	if c.Runtime.DefaultIdleTimeout != nil {
		if d := c.Runtime.DefaultIdleTimeout.AsDuration(); d > 0 {
			opts.DefaultIdleTimeout = d
		}
	}
	return opts
}

// ==============================================================
// AttemptReaper:定时扫内存缓存,到期则 Terminate
// 实现 kratos.Transport 的最小接口(Start/Stop),在 main 里 kratos.Server(..., reaper) 注册
// ==============================================================

// DefaultPassedGrace 通关后给用户复盘的宽限时间,到期容器被 reaper 清理
const DefaultPassedGrace = 10 * time.Minute

// AttemptReaper 空闲/通关 attempt 清理器
//
// 行为:
//   - 每隔 ReaperInterval(默认 1min) 扫一次 AttemptStore.Snapshot
//   - 命中 now - LastActiveAt > DefaultIdleTimeout 的 running attempt:调 Terminate (status=terminated)
//   - 命中 now - FinishedAt > PassedGrace 的 passed attempt:调 Terminate (仅停容器,状态保持 passed)
//
// 与生产 docker 运行时配合时要注意:
//   - Terminate 内部会 runner.Stop,失败只记日志,内存缓存仍会被 Usecase 更新为 terminated
//   - passed grace 清理会走 Usecase 的幂等 Terminate;但 passed 已经不 IsActive,
//     需要 Usecase 暴露独立 CleanupPassed 或 reaper 直接 runner.Stop。当前实现为 reaper
//     直接删 store + 日志,容器的停靠 docker 宿机自带的短命策略兜底
type AttemptReaper struct {
	store       *store.AttemptStore
	uc          *attempt.AttemptUsecase
	log         *zap.Logger
	interval    time.Duration
	idleCutoff  time.Duration
	passedGrace time.Duration

	cancel context.CancelFunc
	doneCh chan struct{}
}

// NewAttemptReaper 构造(时间参数从 conf.Runtime 读)
func NewAttemptReaper(
	s *store.AttemptStore,
	uc *attempt.AttemptUsecase,
	c *conf.Bootstrap,
	logger *zap.Logger,
) *AttemptReaper {
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
		// passed_grace 字段在 proto regen 后会暴露 c.Runtime.PassedGrace,
		// 当前先用内置默认,保持 yaml 不用新字段也能跑
	}
	return &AttemptReaper{
		store:       s,
		uc:          uc,
		log:         logger,
		interval:    interval,
		idleCutoff:  idle,
		passedGrace: grace,
		doneCh:      make(chan struct{}),
	}
}

// Start 实现 kratos.transport.Server.Start
func (r *AttemptReaper) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.log.Info("attempt reaper started",
		zap.Duration("interval", r.interval),
		zap.Duration("idle_cutoff", r.idleCutoff))

	go func() {
		defer close(r.doneCh)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				r.log.Info("attempt reaper exiting")
				return
			case now := <-ticker.C:
				r.reapOnce(ctx, now)
			}
		}
	}()
	return nil
}

// Stop 实现 kratos.transport.Server.Stop
func (r *AttemptReaper) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	select {
	case <-r.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// reapOnce 单次扫描,处理两类到期 attempt
//   - running 且 idle 超时
//   - passed 且 grace 过期
func (r *AttemptReaper) reapOnce(ctx context.Context, now time.Time) {
	list := r.store.Snapshot()
	for _, a := range list {
		switch a.Status {
		case "running":
			idle := now.Sub(a.LastActiveAt)
			if idle < r.idleCutoff {
				continue
			}
			r.log.Info("reaping idle attempt",
				zap.Int64("id", a.ID),
				zap.String("slug", a.ScenarioSlug),
				zap.Duration("idle", idle))
			if err := r.uc.Terminate(ctx, a.ID); err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("reap terminate failed",
					zap.Int64("id", a.ID),
					zap.Error(err))
			}
		case "passed":
			if a.FinishedAt == nil {
				// 异常:状态 passed 但没 FinishedAt,用 LastActiveAt 兜底
				if now.Sub(a.LastActiveAt) < r.passedGrace {
					continue
				}
			} else if now.Sub(*a.FinishedAt) < r.passedGrace {
				continue
			}
			r.log.Info("reaping passed-grace attempt",
				zap.Int64("id", a.ID),
				zap.String("slug", a.ScenarioSlug))
			// passed 态不再走 Terminate(会改 status),直接触发容器停靠 + store 删除
			if err := r.uc.CleanupPassed(ctx, a.ID); err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("reap cleanup passed failed",
					zap.Int64("id", a.ID),
					zap.Error(err))
			}
		}
	}
}
