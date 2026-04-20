package server

import (
	"github.com/google/wire"
	"go.uber.org/zap"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(
	NewGRPCServer, NewHTTPServer, NewWebSocketServer, NewWebSocketApp)


// NewWebSocketAppWrapper new a WebSocket app.
func NewWebSocketAppWrapper(ws *WebSocketServer, logger *zap.Logger) *WebSocketApp {
	return NewWebSocketApp(ws, logger)
}
