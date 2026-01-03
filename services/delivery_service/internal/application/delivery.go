package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

// SyncUseCaseImpl 同步用例实现
type SyncUseCaseImpl struct {
	syncStateRepo out.SyncStateRepository
	messageRepo   out.MessageQueryRepository
	inboxRepo     out.InboxQueryRepository
	connManager   out.ConnectionManager
}

func NewSyncUseCase(
	syncStateRepo out.SyncStateRepository,
	messageRepo out.MessageQueryRepository,
	inboxRepo out.InboxQueryRepository,
	connManager out.ConnectionManager,
) in.SyncUseCase {
	return &SyncUseCaseImpl{
		syncStateRepo: syncStateRepo,
		messageRepo:   messageRepo,
		inboxRepo:     inboxRepo,
		connManager:   connManager,
	}
}

// SyncMessages 同步消息（增量拉取）
// 实现推拉结合：在线时实时推送，离线/重连时基于 lastAckSeq 增量拉取
func (uc *SyncUseCaseImpl) SyncMessages(ctx context.Context, req *in.SyncRequest) (*in.SyncResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	response := &in.SyncResponse{
		Messages:   make(map[uint64][]*in.SyncMessage),
		HasMore:    make(map[uint64]bool),
		LatestSeqs: make(map[uint64]uint64),
	}

	// 获取用户的同步状态
	syncState, err := uc.syncStateRepo.GetSyncState(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get sync state failed: %w", err)
	}

	// 合并请求中的同步点和存储的同步状态
	syncPoints := make(map[uint64]uint64)
	if syncState != nil {
		for convID, seq := range syncState.ConversationAckSeqs {
			syncPoints[convID] = seq
		}
	}
	// 请求中的同步点优先级更高
	for convID, seq := range req.SyncPoints {
		if existingSeq, ok := syncPoints[convID]; !ok || seq > existingSeq {
			syncPoints[convID] = seq
		}
	}

	// 如果没有指定同步点，获取用户的所有会话
	if len(syncPoints) == 0 {
		convIDs, err := uc.inboxRepo.GetUserConversationIDs(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user conversations failed: %w", err)
		}
		for _, convID := range convIDs {
			syncPoints[convID] = 0
		}
	}

	// 并发获取各会话的消息
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(syncPoints))

	for convID, afterSeq := range syncPoints {
		wg.Add(1)
		go func(conversationID, lastSeq uint64) {
			defer wg.Done()

			// 获取消息
			messages, err := uc.messageRepo.GetMessagesAfterSeq(ctx, conversationID, lastSeq, req.Limit+1)
			if err != nil {
				errChan <- fmt.Errorf("get messages for conv %d failed: %w", conversationID, err)
				return
			}

			// 获取最新序号
			latestSeq, err := uc.messageRepo.GetLatestSeq(ctx, conversationID)
			if err != nil {
				errChan <- fmt.Errorf("get latest seq for conv %d failed: %w", conversationID, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()

			// 判断是否有更多
			hasMore := len(messages) > req.Limit
			if hasMore {
				messages = messages[:req.Limit]
			}

			// 转换消息格式
			syncMessages := make([]*in.SyncMessage, len(messages))
			for i, msg := range messages {
				syncMessages[i] = &in.SyncMessage{
					ID:             msg.ID,
					ConversationID: msg.ConversationID,
					SenderID:       msg.SenderID,
					Seq:            msg.Seq,
					ContentType:    msg.ContentType,
					Content:        msg.Content,
					Status:         msg.Status,
					CreatedAt:      msg.CreatedAt,
				}
			}

			response.Messages[conversationID] = syncMessages
			response.HasMore[conversationID] = hasMore
			response.LatestSeqs[conversationID] = latestSeq

		}(convID, afterSeq)
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	// 更新最后同步时间
	if err := uc.syncStateRepo.UpdateLastSyncTime(ctx, req.UserID, time.Now().Unix()); err != nil {
		// 记录日志但不阻塞
		fmt.Printf("update last sync time failed: %v\n", err)
	}

	return response, nil
}

// AckMessages 确认消息已收到
func (uc *SyncUseCaseImpl) AckMessages(ctx context.Context, userID, conversationID, ackSeq uint64) error {
	// 更新 ACK 序号
	if err := uc.syncStateRepo.UpdateAckSeq(ctx, userID, conversationID, ackSeq); err != nil {
		return fmt.Errorf("update ack seq failed: %w", err)
	}

	// 更新收件箱已读位置
	if err := uc.inboxRepo.UpdateLastRead(ctx, userID, conversationID, ackSeq); err != nil {
		return fmt.Errorf("update inbox read seq failed: %w", err)
	}

	return nil
}

// GetUnreadConversations 获取有未读消息的会话列表
func (uc *SyncUseCaseImpl) GetUnreadConversations(ctx context.Context, userID uint64) ([]*in.UnreadConversation, error) {
	inboxes, err := uc.inboxRepo.GetUserInboxes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user inboxes failed: %w", err)
	}

	unreadConvs := make([]*in.UnreadConversation, 0)
	for _, inbox := range inboxes {
		if inbox.UnreadCount > 0 {
			unreadConvs = append(unreadConvs, &in.UnreadConversation{
				ConversationID: inbox.ConversationID,
				UnreadCount:    inbox.UnreadCount,
				LastMsgSeq:     inbox.LastDeliveredSeq,
				LastMsgTime:    inbox.LastMsgTime,
				LastAckSeq:     inbox.LastReadSeq,
			})
		}
	}

	return unreadConvs, nil
}

// GetSyncState 获取同步状态
func (uc *SyncUseCaseImpl) GetSyncState(ctx context.Context, userID uint64) (*in.SyncState, error) {
	state, err := uc.syncStateRepo.GetSyncState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get sync state failed: %w", err)
	}

	if state == nil {
		return &in.SyncState{
			UserID:              userID,
			ConversationAckSeqs: make(map[uint64]uint64),
			TotalUnread:         0,
			LastSyncAt:          0,
		}, nil
	}

	// 计算总未读数
	totalUnread, err := uc.inboxRepo.GetTotalUnread(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get total unread failed: %w", err)
	}

	return &in.SyncState{
		UserID:              userID,
		ConversationAckSeqs: state.ConversationAckSeqs,
		TotalUnread:         totalUnread,
		LastSyncAt:          state.LastSyncAt,
	}, nil
}

// AckUseCaseImpl ACK用例实现
type AckUseCaseImpl struct {
	pendingAckRepo out.PendingAckRepository
	syncStateRepo  out.SyncStateRepository
	connManager    out.ConnectionManager
	resendInterval time.Duration
	maxRetry       int
}

func NewAckUseCase(
	pendingAckRepo out.PendingAckRepository,
	syncStateRepo out.SyncStateRepository,
	connManager out.ConnectionManager,
) in.AckUseCase {
	return &AckUseCaseImpl{
		pendingAckRepo: pendingAckRepo,
		syncStateRepo:  syncStateRepo,
		connManager:    connManager,
		resendInterval: 30 * time.Second,
		maxRetry:       3,
	}
}

// MessageAck 消息ACK
func (uc *AckUseCaseImpl) MessageAck(ctx context.Context, userID, conversationID, messageID, seq uint64) error {
	// 从待确认列表中移除
	if err := uc.pendingAckRepo.Remove(ctx, userID, messageID); err != nil {
		return fmt.Errorf("remove pending ack failed: %w", err)
	}

	// 更新同步状态
	if err := uc.syncStateRepo.UpdateAckSeq(ctx, userID, conversationID, seq); err != nil {
		return fmt.Errorf("update ack seq failed: %w", err)
	}

	return nil
}

// BatchMessageAck 批量消息ACK
func (uc *AckUseCaseImpl) BatchMessageAck(ctx context.Context, userID uint64, acks []*in.MessageAckItem) error {
	// 按会话分组
	convAcks := make(map[uint64]uint64) // conversationID -> maxSeq
	messageIDs := make([]uint64, 0, len(acks))

	for _, ack := range acks {
		messageIDs = append(messageIDs, ack.MessageID)
		if existingSeq, ok := convAcks[ack.ConversationID]; !ok || ack.Seq > existingSeq {
			convAcks[ack.ConversationID] = ack.Seq
		}
	}

	// 批量移除待确认
	if err := uc.pendingAckRepo.BatchRemove(ctx, userID, messageIDs); err != nil {
		return fmt.Errorf("batch remove pending acks failed: %w", err)
	}

	// 批量更新同步状态
	for convID, seq := range convAcks {
		if err := uc.syncStateRepo.UpdateAckSeq(ctx, userID, convID, seq); err != nil {
			return fmt.Errorf("update ack seq for conv %d failed: %w", convID, err)
		}
	}

	return nil
}

// GetPendingAcks 获取待确认的消息
func (uc *AckUseCaseImpl) GetPendingAcks(ctx context.Context, userID uint64) ([]*in.PendingAck, error) {
	pendingAcks, err := uc.pendingAckRepo.GetPending(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get pending acks failed: %w", err)
	}

	result := make([]*in.PendingAck, len(pendingAcks))
	for i, pa := range pendingAcks {
		result[i] = &in.PendingAck{
			MessageID:      pa.MessageID,
			ConversationID: pa.ConversationID,
			Seq:            pa.Seq,
			SentAt:         pa.SentAt.Unix(),
			RetryCount:     pa.RetryCount,
		}
	}

	return result, nil
}

// ResendUnacked 重发未确认的消息
func (uc *AckUseCaseImpl) ResendUnacked(ctx context.Context, userID uint64) error {
	pendingAcks, err := uc.pendingAckRepo.GetPending(ctx, userID)
	if err != nil {
		return fmt.Errorf("get pending acks failed: %w", err)
	}

	now := time.Now()
	for _, pa := range pendingAcks {
		// 检查是否需要重发
		if now.Sub(pa.SentAt) < uc.resendInterval {
			continue
		}

		// 检查重试次数
		if pa.RetryCount >= uc.maxRetry {
			// 超过最大重试次数，标记为失败
			if err := uc.pendingAckRepo.MarkFailed(ctx, userID, pa.MessageID); err != nil {
				fmt.Printf("mark pending ack as failed error: %v\n", err)
			}
			continue
		}

		// 重发消息
		payload, _ := json.Marshal(map[string]interface{}{
			"type": "message_resend",
			"data": map[string]interface{}{
				"message_id":      pa.MessageID,
				"conversation_id": pa.ConversationID,
				"seq":             pa.Seq,
			},
		})

		if err := uc.connManager.Send(userID, payload); err != nil {
			fmt.Printf("resend message failed: %v\n", err)
			continue
		}

		// 更新重试次数和发送时间
		if err := uc.pendingAckRepo.IncrRetry(ctx, userID, pa.MessageID); err != nil {
			fmt.Printf("incr retry count failed: %v\n", err)
		}
	}

	return nil
}

// DeliveryUseCaseImpl 投递用例实现（增强版）
type DeliveryUseCaseImpl struct {
	onlineUserRepo out.OnlineUserRepository
	pendingMsgRepo out.PendingMessageRepository
	pendingAckRepo out.PendingAckRepository
	connManager    out.ConnectionManager
	pushService    out.PushService
}

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
	}
}

// SetPendingAckRepo 设置待确认仓储
func (uc *DeliveryUseCaseImpl) SetPendingAckRepo(repo out.PendingAckRepository) {
	uc.pendingAckRepo = repo
}

// DeliverMessage 投递消息
func (uc *DeliveryUseCaseImpl) DeliverMessage(ctx context.Context, event *entity.MessageEvent) error {
	// 构建消息载荷
	payload, err := json.Marshal(map[string]interface{}{
		"type": event.Type,
		"data": map[string]interface{}{
			"message_id":      event.MessageID,
			"conversation_id": event.ConversationID,
			"sender_id":       event.SenderID,
			"seq":             event.Seq,
			"content_type":    event.ContentType,
			"content":         event.Content,
			"created_at":      event.CreatedAt.Unix(),
		},
	})
	if err != nil {
		return fmt.Errorf("marshal message payload failed: %w", err)
	}

	// 批量获取接收者在线状态
	onlineUsers, err := uc.onlineUserRepo.GetOnlineUsers(ctx, event.ReceiverIDs)
	if err != nil {
		return fmt.Errorf("get online users failed: %w", err)
	}

	// 分流处理：在线推送，离线入库
	for _, receiverID := range event.ReceiverIDs {
		if receiverID == event.SenderID {
			continue // 跳过发送者自己
		}

		if devices, ok := onlineUsers[receiverID]; ok && len(devices) > 0 {
			// 在线：直接推送
			if err := uc.deliverToOnlineUser(ctx, receiverID, event, payload); err != nil {
				// 推送失败，转入离线队列
				uc.saveForOffline(ctx, receiverID, event, payload)
			}
		} else {
			// 离线：保存待投递消息
			uc.saveForOffline(ctx, receiverID, event, payload)

			// 发送离线推送通知
			if uc.pushService != nil {
				uc.sendPushNotification(ctx, receiverID, event)
			}
		}
	}

	return nil
}

// deliverToOnlineUser 投递给在线用户
func (uc *DeliveryUseCaseImpl) deliverToOnlineUser(ctx context.Context, userID uint64, event *entity.MessageEvent, payload []byte) error {
	// 发送消息
	if err := uc.connManager.Send(userID, payload); err != nil {
		return err
	}

	// 记录待确认（等待客户端ACK）
	if uc.pendingAckRepo != nil {
		pendingAck := &entity.PendingAckItem{
			UserID:         userID,
			MessageID:      event.MessageID,
			ConversationID: event.ConversationID,
			Seq:            event.Seq,
			SentAt:         time.Now(),
			RetryCount:     0,
		}
		if err := uc.pendingAckRepo.Save(ctx, pendingAck); err != nil {
			fmt.Printf("save pending ack failed: %v\n", err)
		}
	}

	return nil
}

// saveForOffline 保存离线消息
func (uc *DeliveryUseCaseImpl) saveForOffline(ctx context.Context, userID uint64, event *entity.MessageEvent, payload []byte) {
	pendingMsg := &entity.PendingMessage{
		UserID:         userID,
		MessageID:      event.MessageID,
		ConversationID: event.ConversationID,
		Payload:        string(payload),
		Status:         entity.DeliveryStatusPending,
		RetryCount:     0,
		CreatedAt:      time.Now(),
	}

	if err := uc.pendingMsgRepo.Save(ctx, pendingMsg); err != nil {
		fmt.Printf("save pending message failed: %v\n", err)
	}
}

// sendPushNotification 发送离线推送通知
func (uc *DeliveryUseCaseImpl) sendPushNotification(ctx context.Context, userID uint64, event *entity.MessageEvent) {
	notification := &entity.PushNotification{
		UserID: userID,
		Title:  "新消息",
		Body:   "您有一条新消息",
		Data: map[string]string{
			"conversation_id": fmt.Sprintf("%d", event.ConversationID),
			"message_id":      fmt.Sprintf("%d", event.MessageID),
		},
	}

	if err := uc.pushService.Push(ctx, notification); err != nil {
		fmt.Printf("send push notification failed: %v\n", err)
	}
}

// DeliverToUser 投递消息给指定用户
func (uc *DeliveryUseCaseImpl) DeliverToUser(ctx context.Context, userID uint64, message []byte) error {
	// 检查用户是否在线
	isOnline, err := uc.onlineUserRepo.IsOnline(ctx, userID)
	if err != nil {
		return fmt.Errorf("check online status failed: %w", err)
	}

	if isOnline {
		return uc.connManager.Send(userID, message)
	}

	// 离线：不处理，由上层决定是否需要持久化
	return nil
}

// ProcessPendingMessages 处理待投递消息（用户重连时调用）
func (uc *DeliveryUseCaseImpl) ProcessPendingMessages(ctx context.Context, userID uint64) error {
	// 获取待投递消息
	pendingMsgs, err := uc.pendingMsgRepo.GetPending(ctx, userID, 100)
	if err != nil {
		return fmt.Errorf("get pending messages failed: %w", err)
	}

	for _, msg := range pendingMsgs {
		// 尝试投递
		if err := uc.connManager.Send(userID, []byte(msg.Payload)); err != nil {
			// 投递失败，增加重试次数
			uc.pendingMsgRepo.IncrRetryCount(ctx, msg.ID)
			continue
		}

		// 投递成功，标记为已投递
		if err := uc.pendingMsgRepo.MarkDelivered(ctx, msg.ID); err != nil {
			fmt.Printf("mark message delivered failed: %v\n", err)
		}
	}

	return nil
}
