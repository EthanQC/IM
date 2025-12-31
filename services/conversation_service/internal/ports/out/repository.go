package out

import (
	"context"

	"github.com/EthanQC/IM/services/conversation_service/internal/domain/entity"
)

// ConversationRepository 会话仓储接口
type ConversationRepository interface {
	// Create 创建会话
	Create(ctx context.Context, conv *entity.Conversation) error

	// GetByID 根据ID获取会话
	GetByID(ctx context.Context, id uint64) (*entity.Conversation, error)

	// Update 更新会话
	Update(ctx context.Context, conv *entity.Conversation) error

	// Delete 删除会话
	Delete(ctx context.Context, id uint64) error

	// GetSingleConversation 获取两个用户之间的单聊会话
	GetSingleConversation(ctx context.Context, userID1, userID2 uint64) (*entity.Conversation, error)

	// ListByUserID 获取用户的会话列表
	ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Conversation, int, error)
}

// ParticipantRepository 会话成员仓储接口
type ParticipantRepository interface {
	// Create 添加成员
	Create(ctx context.Context, p *entity.Participant) error

	// CreateBatch 批量添加成员
	CreateBatch(ctx context.Context, participants []*entity.Participant) error

	// GetByID 根据ID获取成员
	GetByID(ctx context.Context, id uint64) (*entity.Participant, error)

	// Get 获取指定会话的指定用户成员信息
	Get(ctx context.Context, conversationID, userID uint64) (*entity.Participant, error)

	// List 获取会话成员列表
	List(ctx context.Context, conversationID uint64) ([]*entity.Participant, error)

	// ListByUserID 获取用户参与的会话ID列表
	ListByUserID(ctx context.Context, userID uint64) ([]uint64, error)

	// Update 更新成员
	Update(ctx context.Context, p *entity.Participant) error

	// Delete 删除成员
	Delete(ctx context.Context, conversationID, userID uint64) error

	// DeleteBatch 批量删除成员
	DeleteBatch(ctx context.Context, conversationID uint64, userIDs []uint64) error

	// Count 获取会话成员数量
	Count(ctx context.Context, conversationID uint64) (int, error)

	// IsMember 检查是否为会话成员
	IsMember(ctx context.Context, conversationID, userID uint64) (bool, error)
}

// EventPublisher 事件发布器接口
type EventPublisher interface {
	// Publish 发布事件
	Publish(ctx context.Context, topic string, event interface{}) error
}
