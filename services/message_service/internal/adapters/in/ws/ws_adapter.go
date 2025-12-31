package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/in"
)

const (
	// 心跳周期
	pingPeriod = 30 * time.Second
	// 读超时
	pongWait = 60 * time.Second
	// 写超时
	writeWait = 10 * time.Second
	// 最大消息大小
	maxMessageSize = 64 * 1024 // 64KB
)

// WSMessage WebSocket消息结构
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	ID   string          `json:"id,omitempty"` // 用于ACK
}

// WSMessageType WebSocket消息类型
const (
	WSTypeSendMessage   = "send_message"
	WSTypeMessageAck    = "message_ack"
	WSTypeNewMessage    = "new_message"
	WSTypeMessageRead   = "message_read"
	WSTypeMessageRevoke = "message_revoke"
	WSTypePing          = "ping"
	WSTypePong          = "pong"
	WSTypeError         = "error"
)

// Client WebSocket客户端连接
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	userID   uint64
	deviceID string
	send     chan []byte
	closed   bool
	mu       sync.Mutex
}

// Hub 管理所有客户端连接
type Hub struct {
	clients       map[uint64]map[string]*Client // userID -> deviceID -> Client
	broadcast     chan []byte
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
	messageUseCase in.MessageUseCase
}

// NewHub 创建Hub
func NewHub(messageUseCase in.MessageUseCase) *Hub {
	return &Hub{
		clients:       make(map[uint64]map[string]*Client),
		broadcast:     make(chan []byte, 256),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		messageUseCase: messageUseCase,
	}
}

// Run 启动Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; !ok {
				h.clients[client.userID] = make(map[string]*Client)
			}
			// 如果同设备已存在连接，先关闭旧连接
			if oldClient, ok := h.clients[client.userID][client.deviceID]; ok {
				oldClient.Close()
			}
			h.clients[client.userID][client.deviceID] = client
			h.mu.Unlock()
			log.Printf("Client registered: userID=%d, deviceID=%s", client.userID, client.deviceID)

		case client := <-h.unregister:
			h.mu.Lock()
			if devices, ok := h.clients[client.userID]; ok {
				if c, ok := devices[client.deviceID]; ok && c == client {
					delete(devices, client.deviceID)
					if len(devices) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: userID=%d, deviceID=%s", client.userID, client.deviceID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, devices := range h.clients {
				for _, client := range devices {
					select {
					case client.send <- message:
					default:
						client.Close()
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToUser 发送消息给指定用户的所有设备
func (h *Hub) SendToUser(userID uint64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if devices, ok := h.clients[userID]; ok {
		for _, client := range devices {
			select {
			case client.send <- message:
			default:
				go client.Close()
			}
		}
	}
}

// SendToUsers 发送消息给多个用户
func (h *Hub) SendToUsers(userIDs []uint64, message []byte) {
	for _, userID := range userIDs {
		h.SendToUser(userID, message)
	}
}

// IsOnline 检查用户是否在线
func (h *Hub) IsOnline(userID uint64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	devices, ok := h.clients[userID]
	return ok && len(devices) > 0
}

// GetOnlineCount 获取在线用户数
func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境应该验证Origin
	},
}

// ServeWs 处理WebSocket连接请求
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request, userID uint64, deviceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		userID:   userID,
		deviceID: deviceID,
		send:     make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
	c.conn.Close()
}

// readPump 读取消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
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

// writePump 写入消息
func (c *Client) writePump() {
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

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("invalid message format")
		return
	}

	switch msg.Type {
	case WSTypeSendMessage:
		c.handleSendMessage(msg)
	case WSTypePing:
		c.sendPong()
	default:
		c.sendError(fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

// handleSendMessage 处理发送消息请求
func (c *Client) handleSendMessage(msg WSMessage) {
	var req struct {
		ConversationID uint64               `json:"conversation_id"`
		ClientMsgID    string               `json:"client_msg_id"`
		ContentType    int8                 `json:"content_type"`
		Content        entity.MessageContent `json:"content"`
		ReplyToMsgID   *uint64              `json:"reply_to_msg_id"`
	}

	if err := json.Unmarshal(msg.Data, &req); err != nil {
		c.sendError("invalid send_message data")
		return
	}

	result, err := c.hub.messageUseCase.SendMessage(context.Background(), &in.SendMessageInput{
		ConversationID: req.ConversationID,
		SenderID:       c.userID,
		ClientMsgID:    req.ClientMsgID,
		ContentType:    entity.MessageContentType(req.ContentType),
		Content:        req.Content,
		ReplyToMsgID:   req.ReplyToMsgID,
	})
	if err != nil {
		c.sendError(err.Error())
		return
	}

	// 发送ACK
	ack := WSMessage{
		Type: WSTypeMessageAck,
		ID:   msg.ID,
	}
	ackData, _ := json.Marshal(map[string]interface{}{
		"client_msg_id": req.ClientMsgID,
		"message_id":    result.ID,
		"seq":           result.Seq,
		"created_at":    result.CreatedAt,
	})
	ack.Data = ackData

	response, _ := json.Marshal(ack)
	c.send <- response
}

// sendError 发送错误消息
func (c *Client) sendError(errMsg string) {
	msg := WSMessage{
		Type: WSTypeError,
	}
	msg.Data, _ = json.Marshal(map[string]string{"error": errMsg})
	data, _ := json.Marshal(msg)
	c.send <- data
}

// sendPong 发送Pong响应
func (c *Client) sendPong() {
	msg := WSMessage{
		Type: WSTypePong,
	}
	data, _ := json.Marshal(msg)
	c.send <- data
}

// NewMessageNotification 新消息通知结构
type NewMessageNotification struct {
	MessageID      uint64               `json:"message_id"`
	ConversationID uint64               `json:"conversation_id"`
	SenderID       uint64               `json:"sender_id"`
	Seq            uint64               `json:"seq"`
	ContentType    int8                 `json:"content_type"`
	Content        entity.MessageContent `json:"content"`
	CreatedAt      time.Time            `json:"created_at"`
}

// NotifyNewMessage 通知新消息
func (h *Hub) NotifyNewMessage(userIDs []uint64, notification *NewMessageNotification) {
	msg := WSMessage{
		Type: WSTypeNewMessage,
	}
	msg.Data, _ = json.Marshal(notification)
	data, _ := json.Marshal(msg)

	h.SendToUsers(userIDs, data)
}

// NotifyMessageRead 通知消息已读
func (h *Hub) NotifyMessageRead(userID uint64, conversationID uint64, readerID uint64, readSeq uint64) {
	msg := WSMessage{
		Type: WSTypeMessageRead,
	}
	msg.Data, _ = json.Marshal(map[string]interface{}{
		"conversation_id": conversationID,
		"reader_id":       readerID,
		"read_seq":        readSeq,
	})
	data, _ := json.Marshal(msg)

	h.SendToUser(userID, data)
}

// NotifyMessageRevoked 通知消息撤回
func (h *Hub) NotifyMessageRevoked(userIDs []uint64, conversationID uint64, messageID uint64) {
	msg := WSMessage{
		Type: WSTypeMessageRevoke,
	}
	msg.Data, _ = json.Marshal(map[string]interface{}{
		"conversation_id": conversationID,
		"message_id":      messageID,
	})
	data, _ := json.Marshal(msg)

	h.SendToUsers(userIDs, data)
}
