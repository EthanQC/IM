package in

import (
	"context"

	"github.com/EthanQC/IM/services/conversation_service/internal/domain/entity"
)

// ConversationUseCase 会话用例接口
type ConversationUseCase interface {
	// CreateConversation 创建会话
	CreateConversation(ctx context.Context, creatorID uint64, convType entity.ConversationType, title string, memberIDs []uint64) (*entity.Conversation, error)

	// GetConversation 获取会话
	GetConversation(ctx context.Context, conversationID uint64) (*entity.Conversation, error)

	// UpdateConversation 更新会话
	UpdateConversation(ctx context.Context, userID, conversationID uint64, title *string, avatarURL *string) (*entity.Conversation, error)

	// DeleteConversation 删除/解散会话
	DeleteConversation(ctx context.Context, userID, conversationID uint64) error

	// GetOrCreateSingleConversation 获取或创建单聊会话
	GetOrCreateSingleConversation(ctx context.Context, userID1, userID2 uint64) (*entity.Conversation, error)

	// ListMyConversations 获取用户的会话列表
	ListMyConversations(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Conversation, int, error)

	// AddMembers 添加成员
	AddMembers(ctx context.Context, operatorID, conversationID uint64, userIDs []uint64) error

	// RemoveMembers 移除成员
	RemoveMembers(ctx context.Context, operatorID, conversationID uint64, userIDs []uint64) error

	// GetMembers 获取会话成员
	GetMembers(ctx context.Context, conversationID uint64) ([]*entity.Participant, error)

	// LeaveConversation 退出会话
	LeaveConversation(ctx context.Context, userID, conversationID uint64) error

	// SetMemberRole 设置成员角色
	SetMemberRole(ctx context.Context, operatorID, conversationID, targetUserID uint64, role entity.ParticipantRole) error

	// MuteMember 禁言成员
	MuteMember(ctx context.Context, operatorID, conversationID, targetUserID uint64, muteSeconds int64) error

	// UnmuteMember 取消禁言
	UnmuteMember(ctx context.Context, operatorID, conversationID, targetUserID uint64) error
}
