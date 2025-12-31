package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 64 * 1024
)

// ConnectionManagerImpl 连接管理器实现
type ConnectionManagerImpl struct {
	connections map[uint64]map[string]*WSConnection // userID -> deviceID -> connection
	mu          sync.RWMutex
	connUseCase in.ConnectionUseCase
}

func NewConnectionManager() out.ConnectionManager {
	return &ConnectionManagerImpl{
		connections: make(map[uint64]map[string]*WSConnection),
	}
}

func (m *ConnectionManagerImpl) SetConnectionUseCase(uc in.ConnectionUseCase) {
	m.connUseCase = uc
}

func (m *ConnectionManagerImpl) Register(userID uint64, deviceID string, conn out.Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.connections[userID]; !ok {
		m.connections[userID] = make(map[string]*WSConnection)
	}

	// 关闭旧连接
	if oldConn, ok := m.connections[userID][deviceID]; ok {
		oldConn.Close()
	}

	wsConn, ok := conn.(*WSConnection)
	if !ok {
		return nil
	}

	m.connections[userID][deviceID] = wsConn
	log.Printf("Connection registered: userID=%d, deviceID=%s", userID, deviceID)
	return nil
}

func (m *ConnectionManagerImpl) Unregister(userID uint64, deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if devices, ok := m.connections[userID]; ok {
		if conn, ok := devices[deviceID]; ok {
			conn.Close()
			delete(devices, deviceID)
			if len(devices) == 0 {
				delete(m.connections, userID)
			}
		}
	}

	log.Printf("Connection unregistered: userID=%d, deviceID=%s", userID, deviceID)
	return nil
}

func (m *ConnectionManagerImpl) GetConnections(userID uint64) []out.Connection {
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

func (m *ConnectionManagerImpl) Send(userID uint64, message []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.connections[userID]
	if !ok {
		return nil
	}

	for _, conn := range devices {
		if err := conn.Send(message); err != nil {
			log.Printf("Failed to send message to user %d: %v", userID, err)
		}
	}
	return nil
}

func (m *ConnectionManagerImpl) SendToDevice(userID uint64, deviceID string, message []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.connections[userID]
	if !ok {
		return nil
	}

	conn, ok := devices[deviceID]
	if !ok {
		return nil
	}

	return conn.Send(message)
}

func (m *ConnectionManagerImpl) Broadcast(userIDs []uint64, message []byte) error {
	for _, userID := range userIDs {
		m.Send(userID, message)
	}
	return nil
}

// WSConnection WebSocket连接
type WSConnection struct {
	conn     *websocket.Conn
	userID   uint64
	deviceID string
	send     chan []byte
	closed   bool
	mu       sync.Mutex
}

func NewWSConnection(conn *websocket.Conn, userID uint64, deviceID string) *WSConnection {
	return &WSConnection{
		conn:     conn,
		userID:   userID,
		deviceID: deviceID,
		send:     make(chan []byte, 256),
	}
}

func (c *WSConnection) UserID() uint64 {
	return c.userID
}

func (c *WSConnection) DeviceID() string {
	return c.deviceID
}

func (c *WSConnection) Send(message []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	select {
	case c.send <- message:
		return nil
	default:
		return nil
	}
}

func (c *WSConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.send)
	return c.conn.Close()
}

// ReadPump 读取消息
func (c *WSConnection) ReadPump(connManager *ConnectionManagerImpl, connUseCase in.ConnectionUseCase) {
	defer func() {
		connManager.Unregister(c.userID, c.deviceID)
		if connUseCase != nil {
			connUseCase.UserDisconnect(nil, c.userID, c.deviceID)
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		// 更新心跳
		if connUseCase != nil {
			connUseCase.Heartbeat(nil, c.userID, c.deviceID)
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump 写入消息
func (c *WSConnection) WritePump() {
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
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WSConnection) handleMessage(data []byte) {
	var msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "ping":
		c.Send([]byte(`{"type":"pong"}`))
	default:
		// 其他消息类型可以在这里处理
	}
}

// WSServer WebSocket服务器
type WSServer struct {
	connManager *ConnectionManagerImpl
	connUseCase in.ConnectionUseCase
	upgrader    websocket.Upgrader
}

func NewWSServer(connManager *ConnectionManagerImpl, connUseCase in.ConnectionUseCase) *WSServer {
	return &WSServer{
		connManager: connManager,
		connUseCase: connUseCase,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// HandleConnection 处理WebSocket连接
func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request, userID uint64, deviceID, platform string) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	wsConn := NewWSConnection(conn, userID, deviceID)
	
	// 注册连接
	s.connManager.Register(userID, deviceID, wsConn)
	
	// 通知用户上线
	if s.connUseCase != nil {
		serverAddr := r.Host // TODO: 获取实际服务器地址
		s.connUseCase.UserConnect(r.Context(), userID, deviceID, platform, serverAddr)
	}

	// 启动读写协程
	go wsConn.WritePump()
	go wsConn.ReadPump(s.connManager, s.connUseCase)
}
