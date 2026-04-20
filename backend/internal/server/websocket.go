package server

import (
	"context"
	"net/http"
	"sync"

	"go.uber.org/zap"
	socketio "github.com/googollee/go-socket.io"
)

type WebSocketServer struct {
	engine *socketio.Server
	log    *zap.Logger
	server *http.Server
	mu     sync.RWMutex
	rooms  map[string]map[string]bool // room -> clientID -> bool
}

func NewWebSocketServer(logger *zap.Logger) *WebSocketServer {
	// 创建Socket.IO服务器
	server := socketio.NewServer(nil)

	ws := &WebSocketServer{
		engine: server,
		log:    logger,
		rooms:  make(map[string]map[string]bool),
	}

	// 配置Socket.IO服务器
	ws.configureServer()
	return ws
}

func (ws *WebSocketServer) configureServer() {
	// 处理连接事件
	ws.engine.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		ws.log.Info("Client connected", zap.String("id", s.ID()))
		return nil
	})

	// 处理断开连接事件
	ws.engine.OnDisconnect("/", func(s socketio.Conn, reason string) {
		ws.log.Info("Client disconnected", zap.String("id", s.ID()), zap.String("reason", reason))
		ws.removeClientFromAllRooms(s.ID())
	})

	// 处理聊天消息事件
	ws.engine.OnEvent("/", "chat_message", func(s socketio.Conn, msg map[string]interface{}) {
		ws.log.Info("Received chat message from client", zap.String("id", s.ID()), zap.Any("message", msg))
		
		// 广播消息到指定房间
		if room, ok := msg["room"].(string); ok {
			msg["sender"] = s.ID()
			ws.engine.BroadcastToRoom("/", room, "chat_message", msg)
		}
	})

	// 处理加入房间事件
	ws.engine.OnEvent("/", "join_room", func(s socketio.Conn, room string) {
		ws.log.Info("Client joining room", zap.String("id", s.ID()), zap.String("room", room))
		s.Join(room)
		ws.addClientToRoom(s.ID(), room)
		
		// 发送确认消息
		s.Emit("joined_room", map[string]string{
			"room": room,
			"id":   s.ID(),
		})
	})

	// 处理离开房间事件
	ws.engine.OnEvent("/", "leave_room", func(s socketio.Conn, room string) {
		ws.log.Info("Client leaving room", zap.String("id", s.ID()), zap.String("room", room))
		s.Leave(room)
		ws.removeClientFromRoom(s.ID(), room)
		
		// 发送确认消息
		s.Emit("left_room", map[string]string{
			"room": room,
			"id":   s.ID(),
		})
	})

	// 处理心跳事件
	ws.engine.OnEvent("/", "ping", func(s socketio.Conn, data interface{}) {
		s.Emit("pong", data)
	})

	// 处理流式消息请求事件
	ws.engine.OnEvent("/", "stream_messages", func(s socketio.Conn, data map[string]interface{}) {
		ws.log.Info("Received stream messages request", zap.Any("data", data))
		
		// 从请求数据中获取sessionId
		sessionId, ok := data["session_id"].(string)
		if !ok {
			s.Emit("stream_error", map[string]string{
				"error": "Missing session_id",
			})
			return
		}
		
		// 模拟流式消息推送
		// 在实际应用中，这里应该调用chat service的StreamMessages方法
		go ws.streamMessages(s, sessionId)
	})
}

func (ws *WebSocketServer) Start(ctx context.Context) error {
	// 创建HTTP服务器
	ws.server = &http.Server{
		Addr:    ":8081", // WebSocket服务器端口
		Handler: ws.engine,
	}

	ws.log.Info("Starting WebSocket server on :8081")
	
	// 启动Socket.IO服务器
	go ws.engine.Serve()
	
	// 启动HTTP服务器
	return ws.server.ListenAndServe()
}

func (ws *WebSocketServer) Stop(ctx context.Context) error {
	ws.log.Info("Stopping WebSocket server")
	
	// 关闭Socket.IO服务器
	ws.engine.Close()
	
	// 关闭HTTP服务器
	if ws.server != nil {
		return ws.server.Shutdown(ctx)
	}
	
	return nil
}

// 添加客户端到房间
func (ws *WebSocketServer) addClientToRoom(clientID, room string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if _, exists := ws.rooms[room]; !exists {
		ws.rooms[room] = make(map[string]bool)
	}
	ws.rooms[room][clientID] = true
}

// 从房间移除客户端
func (ws *WebSocketServer) removeClientFromRoom(clientID, room string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if clients, exists := ws.rooms[room]; exists {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(ws.rooms, room)
		}
	}
}

// 从所有房间移除客户端
func (ws *WebSocketServer) removeClientFromAllRooms(clientID string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	for room, clients := range ws.rooms {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(ws.rooms, room)
		}
	}
}

// 获取房间中的客户端数量
func (ws *WebSocketServer) GetRoomClientCount(room string) int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	if clients, exists := ws.rooms[room]; exists {
		return len(clients)
	}
	return 0
}

// 获取所有房间信息
func (ws *WebSocketServer) GetAllRooms() map[string]int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	rooms := make(map[string]int)
	for room, clients := range ws.rooms {
		rooms[room] = len(clients)
	}
	return rooms
}

// 流式消息推送
func (ws *WebSocketServer) streamMessages(s socketio.Conn, sessionId string) {
	// 使用chat service发送流式消息
	// ws.chat.StreamMessagesToWebSocket(sessionId, 
	// 	func(message map[string]interface{}) {
	// 		s.Emit("stream_message", message)
	// 	},
	// 	func() {
	// 		s.Emit("stream_complete", map[string]string{
	// 			"session_id": sessionId,
	// 			"message": "Stream completed",
	// 		})
	// 	})
}