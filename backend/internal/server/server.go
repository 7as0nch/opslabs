package server

import (
	"github.com/google/wire"
	"go.uber.org/zap"
)

// ProviderSet is server providers.
//
// Round 6 起:
//   - AttemptBootstrapper 删除(AttemptStore 迁 Redis 不再需要 DB 回灌)
//   - runner.Reconcile 启动期清理合并到 GCServer.Start
//   - AttemptReaper / NewAttemptReaper 旧别名清理,全量走 NewGCServer
var ProviderSet = wire.NewSet(
	NewGRPCServer,
	NewHTTPServer,
	NewWebSocketServer,
	NewWebSocketApp,

	// opslabs 运行时装配
	NewScenarioRegistry,
	NewAttemptStore,
	NewRunner,
	NewOpslabsServiceOptions,
	NewGCServer,
)

// NewWebSocketAppWrapper new a WebSocket app.
func NewWebSocketAppWrapper(ws *WebSocketServer, logger *zap.Logger) *WebSocketApp {
	return NewWebSocketApp(ws, logger)
}
