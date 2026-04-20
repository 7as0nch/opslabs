package server

import (
	"context"

	"go.uber.org/zap"
)

// WebSocketApp wraps the WebSocket server to implement the kratos.Server interface
type WebSocketApp struct {
	*WebSocketServer
	log *zap.Logger
}

// NewWebSocketApp creates a new WebSocketApp
func NewWebSocketApp(ws *WebSocketServer, logger *zap.Logger) *WebSocketApp {
	return &WebSocketApp{
		WebSocketServer: ws,
		log:             logger,
	}
}

// Start starts the WebSocket server
func (w *WebSocketApp) Start(ctx context.Context) error {
	w.log.Info("Starting WebSocket application")
	return w.WebSocketServer.Start(ctx)
}

// Stop stops the WebSocket server
func (w *WebSocketApp) Stop(ctx context.Context) error {
	w.log.Info("Stopping WebSocket application")
	return w.WebSocketServer.Stop(ctx)
}