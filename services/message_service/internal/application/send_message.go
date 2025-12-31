package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/in"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

var (
	ErrMessageNotFound     = errors.New("message not found")
	ErrNotMessageSender    = errors.New("not the message sender")
	ErrCannotRevokeMessage = errors.New("cannot revoke message after 2 minutes")
	ErrDuplicateMessage    = errors.New("duplicate message")
)

// MessageUseCaseImpl 消息用例实现
type MessageUseCaseImpl struct {
	msgRepo     out.MessageRepository
	seqRepo     out.SequenceRepository
	inboxRepo   out.InboxRepository
	eventPub    out.EventPublisher
}

var _ in.MessageUseCase = (*MessageUseCaseImpl)(nil)

func NewMessageUseCaseImpl(
	msgRepo out.MessageRepository,
	seqRepo out.SequenceRepository,
	inboxRepo out.InboxRepository,
	eventPub out.EventPublisher,
) *MessageUseCaseImpl {
	return &MessageUseCaseImpl{
		msgRepo:   msgRepo,
		seqRepo:   seqRepo,
		inboxRepo: inboxRepo,
		eventPub:  eventPub,
	}
}

func (uc *MessageUseCaseImpl) SendMessage(ctx context.Context, req *in.SendMessageRequest) (*entity.Message, error) {
	// 幂等检查
	existingMsg, err := uc.msgRepo.GetByClientMsgID(ctx, req.SenderID, req.ClientMsgID)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existingMsg != nil {
		return existingMsg, nil // 返回已存在的消息，实现幂等
	}

	// 获取下一个序号
	seq, err := uc.seqRepo.GetNextSeq(ctx, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("get next seq: %w", err)
	}

	// 创建消息
	now := time.Now()
	msg := &entity.Message{
		ConversationID: req.ConversationID,
		SenderID:       req.SenderID,
		ClientMsgID:    req.ClientMsgID,
		Seq:            seq,
		ContentType:    req.ContentType,
		Content:        req.Content,
		Status:         entity.MessageStatusNormal,
		ReplyToMsgID:   req.ReplyToMsgID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := uc.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	// 发布消息发送事件
	if uc.eventPub != nil {
		contentBytes, _ := json.Marshal(msg.Content)
		event := &out.MessageSentEvent{
			MessageID:      msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			Seq:            msg.Seq,
			ContentType:    int8(msg.ContentType),
			Content:        string(contentBytes),
			CreatedAt:      msg.CreatedAt.Unix(),
		}
		if err := uc.eventPub.PublishMessageSent(ctx, event); err != nil {
			// 记录日志但不阻塞
			fmt.Printf("publish message sent event failed: %v\n", err)
		}
	}

	return msg, nil
}

func (uc *MessageUseCaseImpl) GetMessage(ctx context.Context, messageID uint64) (*entity.Message, error) {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return nil, ErrMessageNotFound
	}
	return msg, nil
}

func (uc *MessageUseCaseImpl) GetHistory(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return uc.msgRepo.GetHistoryAfter(ctx, conversationID, afterSeq, limit)
}

func (uc *MessageUseCaseImpl) GetHistoryBefore(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return uc.msgRepo.GetHistoryBefore(ctx, conversationID, beforeSeq, limit)
}

func (uc *MessageUseCaseImpl) UpdateRead(ctx context.Context, userID, conversationID, readSeq uint64) error {
	if err := uc.inboxRepo.UpdateLastRead(ctx, userID, conversationID, readSeq); err != nil {
		return fmt.Errorf("update last read: %w", err)
	}

	// 清除未读数
	if err := uc.inboxRepo.ClearUnread(ctx, userID, conversationID); err != nil {
		return fmt.Errorf("clear unread: %w", err)
	}

	// 发布已读事件
	if uc.eventPub != nil {
		event := &out.MessageReadEvent{
			UserID:         userID,
			ConversationID: conversationID,
			ReadSeq:        readSeq,
			ReadAt:         time.Now().Unix(),
		}
		if err := uc.eventPub.PublishMessageRead(ctx, event); err != nil {
			fmt.Printf("publish message read event failed: %v\n", err)
		}
	}

	return nil
}

func (uc *MessageUseCaseImpl) RevokeMessage(ctx context.Context, userID, messageID uint64) error {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	// 检查是否为发送者
	if msg.SenderID != userID {
		return ErrNotMessageSender
	}

	// 检查是否可以撤回
	if !msg.CanRevoke() {
		return ErrCannotRevokeMessage
	}

	msg.Revoke()
	if err := uc.msgRepo.Update(ctx, msg); err != nil {
		return fmt.Errorf("update message: %w", err)
	}

	// 发布撤回事件
	if uc.eventPub != nil {
		event := &out.MessageRevokedEvent{
			MessageID:      msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			RevokedAt:      time.Now().Unix(),
		}
		if err := uc.eventPub.PublishMessageRevoked(ctx, event); err != nil {
			fmt.Printf("publish message revoked event failed: %v\n", err)
		}
	}

	return nil
}

func (uc *MessageUseCaseImpl) DeleteMessage(ctx context.Context, userID, messageID uint64) error {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	// 删除消息（这里简化处理，实际应该是软删除，且只对当前用户不可见）
	msg.Delete()
	if err := uc.msgRepo.Update(ctx, msg); err != nil {
		return fmt.Errorf("update message: %w", err)
	}

	return nil
}

func (uc *MessageUseCaseImpl) GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error) {
	return uc.inboxRepo.GetUnreadCount(ctx, userID, conversationID)
}
