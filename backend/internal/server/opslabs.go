/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Opslabs 相关服务的 server 层装配:
 *                 - 场景注册表 / 内存缓存 / 运行时实例工厂
 *                 - AttemptService 的小配置(从 conf.yaml 解析)
 *               放在 server/ 和 http.go、grpc.go 同层,由 main 注册运行
 *
 *               GC / Reaper 周期任务已独立到 gcserver.go(README Day 6 规划的
 *               task.GCServer 位置),这里不再重复定义,避免一个文件扛五项职责
**/
package server

import (
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/db"
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
// Attempt 共享缓存(Round 6 起走 Redis)
// ==============================================================

// NewAttemptStore 构造全局唯一的 Attempt 共享缓存
//
// Round 6 起 Redis 是强依赖:nil 会在 store.NewAttemptStore 里 log.Fatal。
// 部署前请确认 configs/config.yaml 的 data.redis.addr 已配置,否则启动失败。
func NewAttemptStore(rdb db.RedisRepo, logger *zap.Logger) *store.AttemptStore {
	return store.NewAttemptStore(rdb, logger)
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

// GCServer / NewGCServer / DefaultPassedGrace 统一在 gcserver.go 定义。
// Round 6 起 AttemptReaper 别名已废弃,所有装配走 NewGCServer。
