package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 写超时
	writeWait = 10 * time.Second
	// Pong等待时间
	pongWait = 60 * time.Second
	// Ping周期（必须小于pongWait）
	pingPeriod = 30 * time.Second
	// 最大消息大小
	maxMessageSize = 64 * 1024
	// 心跳超时
	heartbeatTimeout = 90 * time.Second
	// 重连检测间隔
	reconnectCheckInterval = 5 * time.Second
)

// WSMessageType WebSocket消息类型
type WSMessageType string

const (
	// 客户端消息类型
	MsgTypePing      WSMessageType = "ping"
	MsgTypeAck       WSMessageType = "ack"
	MsgTypeBatchAck  WSMessageType = "batch_ack"
	MsgTypeSync      WSMessageType = "sync"
	MsgTypeSignaling WSMessageType = "signaling" // WebRTC信令

	// 服务端消息类型
	MsgTypePong       WSMessageType = "pong"
	MsgTypeMessage    WSMessageType = "message"
	MsgTypeNotify     WSMessageType = "notify"
	MsgTypeSyncResp   WSMessageType = "sync_resp"
	MsgTypeSignalResp WSMessageType = "signal_resp"
	MsgTypeError      WSMessageType = "error"
)

// WSMessage WebSocket消息
type WSMessage struct {
	Type WSMessageType   `json:"type"`
	ID   string          `json:"id,omitempty"` // 消息ID，用于ACK
	Data json.RawMessage `json:"data,omitempty"`
	Ts   int64           `json:"ts,omitempty"` // 时间戳
}

// AckData ACK数据
type AckData struct {
	ConversationID uint64 `json:"conversation_id"`
	MessageID      uint64 `json:"message_id"`
	Seq            uint64 `json:"seq"`
}

// BatchAckData 批量ACK数据
type BatchAckData struct {
	Acks []AckData `json:"acks"`
}

// SyncData 同步请求数据
type SyncData struct {
	SyncPoints map[uint64]uint64 `json:"sync_points"` // conversationID -> lastAckSeq
	Limit      int               `json:"limit"`
}

// EnhancedWSConnection 增强版WebSocket连接
type EnhancedWSConnection struct {
	conn        *websocket.Conn
	userID      uint64
	deviceID    string
	platform    string
	serverAddr  string
	send        chan []byte
	closed      int32
	mu          sync.Mutex
	lastPingAt  time.Time
	lastPongAt  time.Time
	connectedAt time.Time

	// 依赖注入
	connManager out.ConnectionManager
	connUseCase in.ConnectionUseCase
	syncUseCase in.SyncUseCase
	ackUseCase  in.AckUseCase
	signalingUC in.SignalingUseCase
}

func NewEnhancedWSConnection(
	conn *websocket.Conn,
	userID uint64,
	deviceID, platform, serverAddr string,
) *EnhancedWSConnection {
	now := time.Now()
	return &EnhancedWSConnection{
		conn:        conn,
		userID:      userID,
		deviceID:    deviceID,
		platform:    platform,
		serverAddr:  serverAddr,
		send:        make(chan []byte, 256),
		lastPingAt:  now,
		lastPongAt:  now,
		connectedAt: now,
	}
}

// SetDependencies 设置依赖
func (c *EnhancedWSConnection) SetDependencies(
	connManager out.ConnectionManager,
	connUseCase in.ConnectionUseCase,
	syncUseCase in.SyncUseCase,
	ackUseCase in.AckUseCase,
	signalingUC in.SignalingUseCase,
) {
	c.connManager = connManager
	c.connUseCase = connUseCase
	c.syncUseCase = syncUseCase
	c.ackUseCase = ackUseCase
	c.signalingUC = signalingUC
}

func (c *EnhancedWSConnection) UserID() uint64 {
	return c.userID
}

func (c *EnhancedWSConnection) DeviceID() string {
	return c.deviceID
}

func (c *EnhancedWSConnection) Send(message []byte) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return fmt.Errorf("connection closed")
	}

	select {
	case c.send <- message:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

func (c *EnhancedWSConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}

	close(c.send)
	return c.conn.Close()
}

func (c *EnhancedWSConnection) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// ReadPump 读取消息
func (c *EnhancedWSConnection) ReadPump() {
	defer func() {
		c.cleanup()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.lastPongAt = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		// 更新心跳
		if c.connUseCase != nil {
			c.connUseCase.Heartbeat(context.Background(), c.userID, c.deviceID)
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				zap.L().Warn("WebSocket error", zap.Uint64("userID", c.userID), zap.Error(err))
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump 写入消息
func (c *EnhancedWSConnection) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				zap.L().Warn("Write error", zap.Uint64("userID", c.userID), zap.Error(err))
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.lastPingAt = time.Now()
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *EnhancedWSConnection) cleanup() {
	// 注销连接
	if c.connManager != nil {
		c.connManager.Unregister(c.userID, c.deviceID)
	}

	// 通知用户离线
	if c.connUseCase != nil {
		c.connUseCase.UserDisconnect(context.Background(), c.userID, c.deviceID)
	}

	zap.L().Info("Connection cleanup",
		zap.Uint64("userID", c.userID),
		zap.String("deviceID", c.deviceID))
}

func (c *EnhancedWSConnection) handleMessage(data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("", "invalid message format")
		return
	}

	ctx := context.Background()

	switch msg.Type {
	case MsgTypePing:
		c.handlePing(msg.ID)

	case MsgTypeAck:
		c.handleAck(ctx, msg.ID, msg.Data)

	case MsgTypeBatchAck:
		c.handleBatchAck(ctx, msg.ID, msg.Data)

	case MsgTypeSync:
		c.handleSync(ctx, msg.ID, msg.Data)

	case MsgTypeSignaling:
		c.handleSignaling(ctx, msg.ID, msg.Data)

	default:
		c.sendError(msg.ID, "unknown message type")
	}
}

func (c *EnhancedWSConnection) handlePing(msgID string) {
	resp := WSMessage{
		Type: MsgTypePong,
		ID:   msgID,
		Ts:   time.Now().UnixMilli(),
	}
	c.sendJSON(resp)
}

func (c *EnhancedWSConnection) handleAck(ctx context.Context, msgID string, data json.RawMessage) {
	if c.ackUseCase == nil {
		c.sendError(msgID, "ack service unavailable")
		return
	}

	var ackData AckData
	if err := json.Unmarshal(data, &ackData); err != nil {
		c.sendError(msgID, "invalid ack data")
		return
	}

	if err := c.ackUseCase.MessageAck(ctx, c.userID, ackData.ConversationID, ackData.MessageID, ackData.Seq); err != nil {
		c.sendError(msgID, err.Error())
		return
	}

	// 发送ACK确认
	c.sendJSON(WSMessage{
		Type: MsgTypeNotify,
		ID:   msgID,
		Data: json.RawMessage(`{"status":"ok"}`),
	})
}

func (c *EnhancedWSConnection) handleBatchAck(ctx context.Context, msgID string, data json.RawMessage) {
	if c.ackUseCase == nil {
		c.sendError(msgID, "ack service unavailable")
		return
	}

	var batchData BatchAckData
	if err := json.Unmarshal(data, &batchData); err != nil {
		c.sendError(msgID, "invalid batch ack data")
		return
	}

	ackItems := make([]*in.MessageAckItem, len(batchData.Acks))
	for i, ack := range batchData.Acks {
		ackItems[i] = &in.MessageAckItem{
			ConversationID: ack.ConversationID,
			MessageID:      ack.MessageID,
			Seq:            ack.Seq,
		}
	}

	if err := c.ackUseCase.BatchMessageAck(ctx, c.userID, ackItems); err != nil {
		c.sendError(msgID, err.Error())
		return
	}

	c.sendJSON(WSMessage{
		Type: MsgTypeNotify,
		ID:   msgID,
		Data: json.RawMessage(`{"status":"ok"}`),
	})
}

func (c *EnhancedWSConnection) handleSync(ctx context.Context, msgID string, data json.RawMessage) {
	if c.syncUseCase == nil {
		c.sendError(msgID, "sync service unavailable")
		return
	}

	var syncData SyncData
	if err := json.Unmarshal(data, &syncData); err != nil {
		c.sendError(msgID, "invalid sync data")
		return
	}

	req := &in.SyncRequest{
		UserID:     c.userID,
		SyncPoints: syncData.SyncPoints,
		Limit:      syncData.Limit,
	}

	resp, err := c.syncUseCase.SyncMessages(ctx, req)
	if err != nil {
		c.sendError(msgID, err.Error())
		return
	}

	respData, _ := json.Marshal(resp)
	c.sendJSON(WSMessage{
		Type: MsgTypeSyncResp,
		ID:   msgID,
		Data: respData,
		Ts:   time.Now().UnixMilli(),
	})
}

func (c *EnhancedWSConnection) handleSignaling(ctx context.Context, msgID string, data json.RawMessage) {
	if c.signalingUC == nil {
		c.sendError(msgID, "signaling service unavailable")
		return
	}

	// 解析信令消息
	var signalMsg struct {
		Action  string          `json:"action"` // offer, answer, ice_candidate, call, accept, reject, hangup
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &signalMsg); err != nil {
		c.sendError(msgID, "invalid signaling data")
		return
	}

	resp, err := c.signalingUC.HandleSignaling(ctx, c.userID, c.deviceID, signalMsg.Action, signalMsg.Payload)
	if err != nil {
		c.sendError(msgID, err.Error())
		return
	}

	respData, _ := json.Marshal(resp)
	c.sendJSON(WSMessage{
		Type: MsgTypeSignalResp,
		ID:   msgID,
		Data: respData,
		Ts:   time.Now().UnixMilli(),
	})
}

func (c *EnhancedWSConnection) sendJSON(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	c.Send(data)
}

func (c *EnhancedWSConnection) sendError(msgID, errMsg string) {
	errData, _ := json.Marshal(map[string]string{"error": errMsg})
	c.sendJSON(WSMessage{
		Type: MsgTypeError,
		ID:   msgID,
		Data: errData,
		Ts:   time.Now().UnixMilli(),
	})
}

// EnhancedConnectionManager 增强版连接管理器
type EnhancedConnectionManager struct {
	connections map[uint64]map[string]*EnhancedWSConnection // userID -> deviceID -> connection
	mu          sync.RWMutex

	// 统计
	totalConns int64
	totalMsgs  int64

	// 依赖
	connUseCase in.ConnectionUseCase
	syncUseCase in.SyncUseCase
	ackUseCase  in.AckUseCase
	signalingUC in.SignalingUseCase
}

func NewEnhancedConnectionManager() *EnhancedConnectionManager {
	return &EnhancedConnectionManager{
		connections: make(map[uint64]map[string]*EnhancedWSConnection),
	}
}

func (m *EnhancedConnectionManager) SetUseCases(
	connUseCase in.ConnectionUseCase,
	syncUseCase in.SyncUseCase,
	ackUseCase in.AckUseCase,
	signalingUC in.SignalingUseCase,
) {
	m.connUseCase = connUseCase
	m.syncUseCase = syncUseCase
	m.ackUseCase = ackUseCase
	m.signalingUC = signalingUC
}

func (m *EnhancedConnectionManager) Register(userID uint64, deviceID string, conn out.Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.connections[userID]; !ok {
		m.connections[userID] = make(map[string]*EnhancedWSConnection)
	}

	// 关闭旧连接
	if oldConn, ok := m.connections[userID][deviceID]; ok {
		oldConn.Close()
		atomic.AddInt64(&m.totalConns, -1)
	}

	enhancedConn, ok := conn.(*EnhancedWSConnection)
	if !ok {
		return fmt.Errorf("invalid connection type")
	}

	m.connections[userID][deviceID] = enhancedConn
	atomic.AddInt64(&m.totalConns, 1)

	zap.L().Info("Connection registered",
		zap.Uint64("userID", userID),
		zap.String("deviceID", deviceID),
		zap.Int64("totalConns", atomic.LoadInt64(&m.totalConns)))

	return nil
}

func (m *EnhancedConnectionManager) Unregister(userID uint64, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if devices, ok := m.connections[userID]; ok {
		if conn, ok := devices[deviceID]; ok {
			conn.Close()
			delete(devices, deviceID)
			atomic.AddInt64(&m.totalConns, -1)

			if len(devices) == 0 {
				delete(m.connections, userID)
			}
		}
	}

	zap.L().Info("Connection unregistered",
		zap.Uint64("userID", userID),
		zap.String("deviceID", deviceID),
		zap.Int64("totalConns", atomic.LoadInt64(&m.totalConns)))

	return nil
}

func (m *EnhancedConnectionManager) GetConnections(userID uint64) []out.Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.connections[userID]
	if !ok {
		return nil
	}

	conns := make([]out.Connection, 0, len(devices))
	for _, conn := range devices {
		conns = append(conns, conn)
	}
	return conns
}

func (m *EnhancedConnectionManager) Send(userID uint64, message []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.connections[userID]
	if !ok {
		return nil
	}

	for _, conn := range devices {
		if err := conn.Send(message); err != nil {
			zap.L().Warn("Failed to send message to user",
				zap.Uint64("userID", userID),
				zap.Error(err))
		}
	}

	atomic.AddInt64(&m.totalMsgs, int64(len(devices)))
	return nil
}

func (m *EnhancedConnectionManager) SendToDevice(userID uint64, deviceID string, message []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.connections[userID]
	if !ok {
		return fmt.Errorf("user not online")
	}

	conn, ok := devices[deviceID]
	if !ok {
		return fmt.Errorf("device not online")
	}

	atomic.AddInt64(&m.totalMsgs, 1)
	return conn.Send(message)
}

func (m *EnhancedConnectionManager) Broadcast(userIDs []uint64, message []byte) error {
	for _, userID := range userIDs {
		m.Send(userID, message)
	}
	return nil
}

// GetStats 获取统计信息
func (m *EnhancedConnectionManager) GetStats() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCount := int64(len(m.connections))
	return map[string]int64{
		"total_connections": atomic.LoadInt64(&m.totalConns),
		"total_messages":    atomic.LoadInt64(&m.totalMsgs),
		"online_users":      userCount,
	}
}

// EnhancedWSServer 增强版WebSocket服务器
type EnhancedWSServer struct {
	connManager *EnhancedConnectionManager
	connUseCase in.ConnectionUseCase
	syncUseCase in.SyncUseCase
	ackUseCase  in.AckUseCase
	signalingUC in.SignalingUseCase
	upgrader    websocket.Upgrader
}

func NewEnhancedWSServer(
	connManager *EnhancedConnectionManager,
	connUseCase in.ConnectionUseCase,
	syncUseCase in.SyncUseCase,
	ackUseCase in.AckUseCase,
	signalingUC in.SignalingUseCase,
) *EnhancedWSServer {
	connManager.SetUseCases(connUseCase, syncUseCase, ackUseCase, signalingUC)

	return &EnhancedWSServer{
		connManager: connManager,
		connUseCase: connUseCase,
		syncUseCase: syncUseCase,
		ackUseCase:  ackUseCase,
		signalingUC: signalingUC,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // 生产环境应该校验Origin
			},
		},
	}
}

// HandleConnection 处理WebSocket连接
func (s *EnhancedWSServer) HandleConnection(w http.ResponseWriter, r *http.Request, userID uint64, deviceID, platform string) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		zap.L().Warn("WebSocket upgrade error", zap.Error(err))
		return
	}

	serverAddr := r.Host
	wsConn := NewEnhancedWSConnection(conn, userID, deviceID, platform, serverAddr)
	wsConn.SetDependencies(s.connManager, s.connUseCase, s.syncUseCase, s.ackUseCase, s.signalingUC)

	// 注册连接
	s.connManager.Register(userID, deviceID, wsConn)

	// 通知用户上线
	if s.connUseCase != nil {
		s.connUseCase.UserConnect(r.Context(), userID, deviceID, platform, serverAddr)
	}

	// 启动读写协程
	go wsConn.WritePump()
	go wsConn.ReadPump()

	// 发送连接成功消息
	welcomeData, _ := json.Marshal(map[string]interface{}{
		"status":      "connected",
		"user_id":     userID,
		"device_id":   deviceID,
		"server_time": time.Now().UnixMilli(),
	})
	wsConn.sendJSON(WSMessage{
		Type: MsgTypeNotify,
		Data: welcomeData,
		Ts:   time.Now().UnixMilli(),
	})
}

// GetStats 获取服务器统计
func (s *EnhancedWSServer) GetStats() map[string]int64 {
	return s.connManager.GetStats()
}
