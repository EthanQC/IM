package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/in"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

// EnhancedMessageUseCaseImpl 增强版消息用例实现
// 支持Redis Timeline热数据缓存和Lua脚本原子序号生成
type EnhancedMessageUseCaseImpl struct {
	msgRepo      out.MessageRepository
	seqRepo      out.SequenceRepository
	inboxRepo    out.InboxRepository
	timelineRepo out.TimelineRepository
	memberRepo   out.ConversationMemberRepository
	eventPub     out.EventPublisher
}

var _ in.MessageUseCase = (*EnhancedMessageUseCaseImpl)(nil)

func NewEnhancedMessageUseCase(
	msgRepo out.MessageRepository,
	seqRepo out.SequenceRepository,
	inboxRepo out.InboxRepository,
	timelineRepo out.TimelineRepository,
	memberRepo out.ConversationMemberRepository,
	eventPub out.EventPublisher,
) *EnhancedMessageUseCaseImpl {
	return &EnhancedMessageUseCaseImpl{
		msgRepo:      msgRepo,
		seqRepo:      seqRepo,
		inboxRepo:    inboxRepo,
		timelineRepo: timelineRepo,
		memberRepo:   memberRepo,
		eventPub:     eventPub,
	}
}

// SendMessage 发送消息
// 1. 幂等检查（基于clientMsgID）
// 2. 使用Redis Lua脚本原子生成序号
// 3. 消息持久化到MySQL
// 4. 消息写入Redis Timeline（热数据缓存）
// 5. 更新收件箱
// 6. 发布Kafka事件
func (uc *EnhancedMessageUseCaseImpl) SendMessage(ctx context.Context, req *in.SendMessageRequest) (*entity.Message, error) {
	if uc.memberRepo == nil {
		return nil, fmt.Errorf("member repository not configured")
	}

	// 幂等检查
	existingMsg, err := uc.msgRepo.GetByClientMsgID(ctx, req.SenderID, req.ClientMsgID)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existingMsg != nil {
		return existingMsg, nil // 返回已存在的消息，实现幂等
	}

	// 获取会话成员并校验身份
	memberIDs, err := uc.memberRepo.ListMemberIDs(ctx, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation members: %w", err)
	}

	isMember := false
	for _, memberID := range memberIDs {
		if memberID == req.SenderID {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, fmt.Errorf("sender not in conversation")
	}

	// 使用Redis Lua脚本原子生成序号
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

	// 持久化到MySQL
	if err := uc.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	// 写入Redis Timeline（热数据缓存）
	if uc.timelineRepo != nil {
		if err := uc.timelineRepo.AddMessage(ctx, req.ConversationID, msg); err != nil {
			// 记录日志但不阻塞
			fmt.Printf("add message to timeline failed: %v\n", err)
		}
	}

	// 更新收件箱
	for _, memberID := range memberIDs {
		if _, err := uc.inboxRepo.GetOrCreate(ctx, memberID, req.ConversationID); err != nil {
			return nil, fmt.Errorf("ensure inbox: %w", err)
		}
		if err := uc.inboxRepo.UpdateLastDelivered(ctx, memberID, req.ConversationID, seq); err != nil {
			return nil, fmt.Errorf("update delivered seq: %w", err)
		}
		if memberID == req.SenderID {
			if err := uc.inboxRepo.UpdateLastRead(ctx, memberID, req.ConversationID, seq); err != nil {
				return nil, fmt.Errorf("update read seq: %w", err)
			}
			if err := uc.inboxRepo.ClearUnread(ctx, memberID, req.ConversationID); err != nil {
				return nil, fmt.Errorf("clear unread: %w", err)
			}
			continue
		}
		if err := uc.inboxRepo.IncrUnread(ctx, memberID, req.ConversationID, 1); err != nil {
			return nil, fmt.Errorf("incr unread: %w", err)
		}
	}

	// 发布消息发送事件到Kafka
	if uc.eventPub != nil {
		contentBytes, _ := json.Marshal(msg.Content)
		event := &out.MessageSentEvent{
			MessageID:      msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			ReceiverIDs:    memberIDs,
			Seq:            msg.Seq,
			ContentType:    int8(msg.ContentType),
			Content:        string(contentBytes),
			CreatedAt:      msg.CreatedAt.Unix(),
		}
		if err := uc.eventPub.PublishMessageSent(ctx, event); err != nil {
			fmt.Printf("publish message sent event failed: %v\n", err)
		}
	}

	return msg, nil
}

// GetMessage 获取单条消息
func (uc *EnhancedMessageUseCaseImpl) GetMessage(ctx context.Context, messageID uint64) (*entity.Message, error) {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return nil, ErrMessageNotFound
	}
	return msg, nil
}

// GetHistory 获取消息历史（优先从Timeline缓存读取）
// 实现推拉结合：优先从Redis Timeline读取热数据，缺失时回源MySQL
func (uc *EnhancedMessageUseCaseImpl) GetHistory(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 优先从Redis Timeline读取
	if uc.timelineRepo != nil {
		messages, err := uc.timelineRepo.GetMessagesAfterSeq(ctx, conversationID, afterSeq, limit)
		if err == nil && len(messages) > 0 {
			return messages, nil
		}
	}

	// Timeline缓存未命中，从MySQL读取
	return uc.msgRepo.GetHistoryAfter(ctx, conversationID, afterSeq, limit)
}

// GetHistoryBefore 获取指定序号之前的消息历史
func (uc *EnhancedMessageUseCaseImpl) GetHistoryBefore(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 优先从Redis Timeline读取
	if uc.timelineRepo != nil {
		messages, err := uc.timelineRepo.GetMessagesBeforeSeq(ctx, conversationID, beforeSeq, limit)
		if err == nil && len(messages) > 0 {
			return messages, nil
		}
	}

	// Timeline缓存未命中，从MySQL读取
	return uc.msgRepo.GetHistoryBefore(ctx, conversationID, beforeSeq, limit)
}

// UpdateRead 更新已读位置
func (uc *EnhancedMessageUseCaseImpl) UpdateRead(ctx context.Context, userID, conversationID, readSeq uint64) error {
	if err := uc.inboxRepo.UpdateLastRead(ctx, userID, conversationID, readSeq); err != nil {
		return fmt.Errorf("update last read: %w", err)
	}

	// 清除未读数
	if err := uc.inboxRepo.ClearUnread(ctx, userID, conversationID); err != nil {
		return fmt.Errorf("clear unread: %w", err)
	}

	// 发布已读事件
	if uc.eventPub != nil {
		receiverIDs := []uint64{}
		if uc.memberRepo != nil {
			members, err := uc.memberRepo.ListMemberIDs(ctx, conversationID)
			if err == nil {
				for _, memberID := range members {
					if memberID == userID {
						continue
					}
					receiverIDs = append(receiverIDs, memberID)
				}
			}
		}
		event := &out.MessageReadEvent{
			UserID:         userID,
			ConversationID: conversationID,
			ReadSeq:        readSeq,
			ReceiverIDs:    receiverIDs,
		}
		if err := uc.eventPub.PublishMessageRead(ctx, event); err != nil {
			fmt.Printf("publish message read event failed: %v\n", err)
		}
	}

	return nil
}

// RevokeMessage 撤回消息
func (uc *EnhancedMessageUseCaseImpl) RevokeMessage(ctx context.Context, userID, messageID uint64) error {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	if msg.SenderID != userID {
		return ErrNotMessageSender
	}

	// 检查是否在可撤回时间内（2分钟）
	if time.Since(msg.CreatedAt) > 2*time.Minute {
		return ErrCannotRevokeMessage
	}

	// 更新消息状态
	msg.Status = entity.MessageStatusRevoked
	msg.UpdatedAt = time.Now()

	if err := uc.msgRepo.Update(ctx, msg); err != nil {
		return fmt.Errorf("update message: %w", err)
	}

	// 发布撤回事件
	if uc.eventPub != nil && uc.memberRepo != nil {
		memberIDs, err := uc.memberRepo.ListMemberIDs(ctx, msg.ConversationID)
		if err == nil {
			event := &out.MessageRevokedEvent{
				MessageID:      msg.ID,
				ConversationID: msg.ConversationID,
				SenderID:       msg.SenderID,
				ReceiverIDs:    memberIDs,
			}
			if err := uc.eventPub.PublishMessageRevoked(ctx, event); err != nil {
				fmt.Printf("publish message revoked event failed: %v\n", err)
			}
		}
	}

	return nil
}

// DeleteMessage 删除消息（仅对自己）
func (uc *EnhancedMessageUseCaseImpl) DeleteMessage(ctx context.Context, userID, messageID uint64) error {
	msg, err := uc.msgRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	// 这里仅更新消息状态为删除，实际删除逻辑可以根据业务需求调整
	msg.Status = entity.MessageStatusDeleted
	msg.UpdatedAt = time.Now()

	return uc.msgRepo.Update(ctx, msg)
}

// GetUnreadCount 获取未读数
func (uc *EnhancedMessageUseCaseImpl) GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error) {
	return uc.inboxRepo.GetUnreadCount(ctx, userID, conversationID)
}
