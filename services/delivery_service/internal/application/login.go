package application

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

// DeliveryUseCaseImpl 消息投递用例实现
type DeliveryUseCaseImpl struct {
	onlineUserRepo     out.OnlineUserRepository
	pendingMsgRepo     out.PendingMessageRepository
	connManager        out.ConnectionManager
	pushService        out.PushService
	maxRetryCount      int
}

// NewDeliveryUseCase 创建消息投递用例
func NewDeliveryUseCase(
	onlineUserRepo out.OnlineUserRepository,
	pendingMsgRepo out.PendingMessageRepository,
	connManager out.ConnectionManager,
	pushService out.PushService,
) in.DeliveryUseCase {
	return &DeliveryUseCaseImpl{
		onlineUserRepo: onlineUserRepo,
		pendingMsgRepo: pendingMsgRepo,
		connManager:    connManager,
		pushService:    pushService,
		maxRetryCount:  3,
	}
}

// DeliverMessage 投递消息给所有接收者
func (uc *DeliveryUseCaseImpl) DeliverMessage(ctx context.Context, event *entity.MessageEvent) error {
	// 获取在线用户
	onlineUsers, err := uc.onlineUserRepo.GetOnlineUsers(ctx, event.ReceiverIDs)
	if err != nil {
		return err
	}

	// 构建推送消息
	payload, _ := json.Marshal(map[string]interface{}{
		"type": "new_message",
		"data": map[string]interface{}{
			"message_id":      event.MessageID,
			"conversation_id": event.ConversationID,
			"sender_id":       event.SenderID,
			"seq":             event.Seq,
			"content_type":    event.ContentType,
			"content":         event.Content,
			"created_at":      event.CreatedAt,
		},
	})

	for _, receiverID := range event.ReceiverIDs {
		// 跳过发送者自己
		if receiverID == event.SenderID {
			continue
		}

		devices, online := onlineUsers[receiverID]
		if online && len(devices) > 0 {
			// 用户在线，通过WebSocket投递
			if err := uc.connManager.Send(receiverID, payload); err != nil {
				log.Printf("Failed to deliver message to user %d: %v", receiverID, err)
				// 投递失败，保存为待投递消息
				uc.savePendingMessage(ctx, receiverID, event, string(payload))
			}
		} else {
			// 用户离线，保存待投递消息并发送推送通知
			uc.savePendingMessage(ctx, receiverID, event, string(payload))
			
			// 发送推送通知
			uc.sendPushNotification(ctx, receiverID, event)
		}
	}

	return nil
}

// savePendingMessage 保存待投递消息
func (uc *DeliveryUseCaseImpl) savePendingMessage(ctx context.Context, userID uint64, event *entity.MessageEvent, payload string) {
	pending := &entity.PendingMessage{
		UserID:         userID,
		MessageID:      event.MessageID,
		ConversationID: event.ConversationID,
		Payload:        payload,
		Status:         entity.DeliveryStatusPending,
		CreatedAt:      time.Now(),
	}
	if err := uc.pendingMsgRepo.Save(ctx, pending); err != nil {
		log.Printf("Failed to save pending message: %v", err)
	}
}

// sendPushNotification 发送推送通知
func (uc *DeliveryUseCaseImpl) sendPushNotification(ctx context.Context, userID uint64, event *entity.MessageEvent) {
	if uc.pushService == nil {
		return
	}

	notification := &entity.PushNotification{
		UserID: userID,
		Title:  "新消息",
		Body:   "您收到一条新消息",
		Data: map[string]string{
			"conversation_id": string(rune(event.ConversationID)),
			"message_id":      string(rune(event.MessageID)),
		},
	}

	if err := uc.pushService.Push(ctx, notification); err != nil {
		log.Printf("Failed to send push notification: %v", err)
	}
}

// DeliverToUser 投递消息给指定用户
func (uc *DeliveryUseCaseImpl) DeliverToUser(ctx context.Context, userID uint64, message []byte) error {
	return uc.connManager.Send(userID, message)
}

// ProcessPendingMessages 处理待投递消息（用户上线时调用）
func (uc *DeliveryUseCaseImpl) ProcessPendingMessages(ctx context.Context, userID uint64) error {
	messages, err := uc.pendingMsgRepo.GetPending(ctx, userID, 100)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		if err := uc.connManager.Send(userID, []byte(msg.Payload)); err != nil {
			// 投递失败，增加重试次数
			uc.pendingMsgRepo.IncrRetryCount(ctx, msg.ID)
			if msg.RetryCount >= uc.maxRetryCount {
				uc.pendingMsgRepo.MarkFailed(ctx, msg.ID)
			}
		} else {
			// 投递成功
			uc.pendingMsgRepo.MarkDelivered(ctx, msg.ID)
		}
	}

	return nil
}
