package conversation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/conversation_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/conversation_service/internal/ports/in"
	"github.com/EthanQC/IM/services/conversation_service/internal/ports/out"
)

var (
	ErrConversationNotFound     = errors.New("conversation not found")
	ErrNotConversationMember    = errors.New("not a conversation member")
	ErrNoPermission             = errors.New("no permission")
	ErrCannotRemoveSelf         = errors.New("cannot remove yourself, use leave instead")
	ErrCannotRemoveOwner        = errors.New("cannot remove owner")
	ErrMemberLimitExceeded      = errors.New("member limit exceeded")
	ErrSingleConvCannotAddMore  = errors.New("single conversation cannot add more members")
	ErrOwnerCannotLeave         = errors.New("owner cannot leave, transfer ownership first")
)

type ConversationUseCaseImpl struct {
	convRepo        out.ConversationRepository
	participantRepo out.ParticipantRepository
	eventPub        out.EventPublisher
}

var _ in.ConversationUseCase = (*ConversationUseCaseImpl)(nil)

func NewConversationUseCaseImpl(
	convRepo out.ConversationRepository,
	participantRepo out.ParticipantRepository,
	eventPub out.EventPublisher,
) *ConversationUseCaseImpl {
	return &ConversationUseCaseImpl{
		convRepo:        convRepo,
		participantRepo: participantRepo,
		eventPub:        eventPub,
	}
}

func (uc *ConversationUseCaseImpl) CreateConversation(ctx context.Context, creatorID uint64, convType entity.ConversationType, title string, memberIDs []uint64) (*entity.Conversation, error) {
	var conv *entity.Conversation

	switch convType {
	case entity.ConversationTypeSingle:
		if len(memberIDs) != 1 {
			return nil, errors.New("single conversation requires exactly one other member")
		}
		// 检查是否已存在单聊会话
		existingConv, err := uc.convRepo.GetSingleConversation(ctx, creatorID, memberIDs[0])
		if err != nil {
			return nil, fmt.Errorf("check existing conversation: %w", err)
		}
		if existingConv != nil {
			return existingConv, nil
		}
		conv = entity.NewSingleConversation()

	case entity.ConversationTypeGroup:
		conv = entity.NewGroupConversation(title, creatorID)

	default:
		return nil, errors.New("invalid conversation type")
	}

	// 创建会话
	if err := uc.convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	// 添加创建者为成员
	creatorRole := entity.ParticipantRoleMember
	if convType == entity.ConversationTypeGroup {
		creatorRole = entity.ParticipantRoleOwner
	}
	creatorParticipant := entity.NewParticipant(conv.ID, creatorID, creatorRole)
	if err := uc.participantRepo.Create(ctx, creatorParticipant); err != nil {
		return nil, fmt.Errorf("add creator as member: %w", err)
	}

	// 添加其他成员
	for _, memberID := range memberIDs {
		if memberID == creatorID {
			continue
		}
		participant := entity.NewParticipant(conv.ID, memberID, entity.ParticipantRoleMember)
		if err := uc.participantRepo.Create(ctx, participant); err != nil {
			return nil, fmt.Errorf("add member %d: %w", memberID, err)
		}
	}

	// 发布事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":            "conversation.created",
			"conversation_id": conv.ID,
			"creator_id":      creatorID,
			"conv_type":       convType,
			"member_ids":      append(memberIDs, creatorID),
			"timestamp":       time.Now(),
		}
		_ = uc.eventPub.Publish(ctx, "conversation-events", event)
	}

	return conv, nil
}

func (uc *ConversationUseCaseImpl) GetConversation(ctx context.Context, conversationID uint64) (*entity.Conversation, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return nil, ErrConversationNotFound
	}
	return conv, nil
}

func (uc *ConversationUseCaseImpl) UpdateConversation(ctx context.Context, userID, conversationID uint64, title *string, avatarURL *string) (*entity.Conversation, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return nil, ErrConversationNotFound
	}

	// 检查权限
	participant, err := uc.participantRepo.Get(ctx, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotConversationMember
	}

	// 只有群聊可以更新，且需要管理员权限
	if conv.IsGroup() && !participant.CanManageMembers() {
		return nil, ErrNoPermission
	}

	conv.Update(title, avatarURL)
	if err := uc.convRepo.Update(ctx, conv); err != nil {
		return nil, fmt.Errorf("update conversation: %w", err)
	}

	return conv, nil
}

func (uc *ConversationUseCaseImpl) DeleteConversation(ctx context.Context, userID, conversationID uint64) error {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return ErrConversationNotFound
	}

	// 检查权限：单聊可以删除，群聊只有群主可以解散
	if conv.IsGroup() {
		participant, err := uc.participantRepo.Get(ctx, conversationID, userID)
		if err != nil {
			return fmt.Errorf("get participant: %w", err)
		}
		if participant == nil || !participant.IsOwner() {
			return ErrNoPermission
		}
	}

	conv.Dissolve()
	if err := uc.convRepo.Update(ctx, conv); err != nil {
		return fmt.Errorf("dissolve conversation: %w", err)
	}

	return nil
}

func (uc *ConversationUseCaseImpl) GetOrCreateSingleConversation(ctx context.Context, userID1, userID2 uint64) (*entity.Conversation, error) {
	// 查找已存在的单聊会话
	conv, err := uc.convRepo.GetSingleConversation(ctx, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("get single conversation: %w", err)
	}
	if conv != nil {
		return conv, nil
	}

	// 创建新的单聊会话
	return uc.CreateConversation(ctx, userID1, entity.ConversationTypeSingle, "", []uint64{userID2})
}

func (uc *ConversationUseCaseImpl) ListMyConversations(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Conversation, int, error) {
	return uc.convRepo.ListByUserID(ctx, userID, page, pageSize)
}

func (uc *ConversationUseCaseImpl) AddMembers(ctx context.Context, operatorID, conversationID uint64, userIDs []uint64) error {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return ErrConversationNotFound
	}

	if conv.IsSingle() {
		return ErrSingleConvCannotAddMore
	}

	// 检查操作者权限
	operator, err := uc.participantRepo.Get(ctx, conversationID, operatorID)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}
	if operator == nil {
		return ErrNotConversationMember
	}

	// 检查成员数量限制
	currentCount, err := uc.participantRepo.Count(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("count members: %w", err)
	}
	if currentCount+len(userIDs) > conv.MemberLimit {
		return ErrMemberLimitExceeded
	}

	// 添加成员
	for _, userID := range userIDs {
		isMember, _ := uc.participantRepo.IsMember(ctx, conversationID, userID)
		if isMember {
			continue
		}
		participant := entity.NewParticipant(conversationID, userID, entity.ParticipantRoleMember)
		if err := uc.participantRepo.Create(ctx, participant); err != nil {
			return fmt.Errorf("add member %d: %w", userID, err)
		}
	}

	// 发布事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":            "conversation.members_added",
			"conversation_id": conversationID,
			"operator_id":     operatorID,
			"added_user_ids":  userIDs,
			"timestamp":       time.Now(),
		}
		_ = uc.eventPub.Publish(ctx, "conversation-events", event)
	}

	return nil
}

func (uc *ConversationUseCaseImpl) RemoveMembers(ctx context.Context, operatorID, conversationID uint64, userIDs []uint64) error {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return ErrConversationNotFound
	}

	// 检查操作者权限
	operator, err := uc.participantRepo.Get(ctx, conversationID, operatorID)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}
	if operator == nil {
		return ErrNotConversationMember
	}
	if !operator.CanManageMembers() {
		return ErrNoPermission
	}

	for _, userID := range userIDs {
		if userID == operatorID {
			return ErrCannotRemoveSelf
		}

		target, _ := uc.participantRepo.Get(ctx, conversationID, userID)
		if target == nil {
			continue
		}
		if target.IsOwner() {
			return ErrCannotRemoveOwner
		}
		// 管理员不能移除管理员
		if operator.IsAdmin() && target.IsAdmin() {
			return ErrNoPermission
		}

		if err := uc.participantRepo.Delete(ctx, conversationID, userID); err != nil {
			return fmt.Errorf("remove member %d: %w", userID, err)
		}
	}

	return nil
}

func (uc *ConversationUseCaseImpl) GetMembers(ctx context.Context, conversationID uint64) ([]*entity.Participant, error) {
	return uc.participantRepo.List(ctx, conversationID)
}

func (uc *ConversationUseCaseImpl) LeaveConversation(ctx context.Context, userID, conversationID uint64) error {
	conv, err := uc.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return ErrConversationNotFound
	}

	participant, err := uc.participantRepo.Get(ctx, conversationID, userID)
	if err != nil {
		return fmt.Errorf("get participant: %w", err)
	}
	if participant == nil {
		return ErrNotConversationMember
	}

	// 群主不能直接退出
	if conv.IsGroup() && participant.IsOwner() {
		return ErrOwnerCannotLeave
	}

	if err := uc.participantRepo.Delete(ctx, conversationID, userID); err != nil {
		return fmt.Errorf("leave conversation: %w", err)
	}

	return nil
}

func (uc *ConversationUseCaseImpl) SetMemberRole(ctx context.Context, operatorID, conversationID, targetUserID uint64, role entity.ParticipantRole) error {
	participant, err := uc.participantRepo.Get(ctx, conversationID, operatorID)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}
	if participant == nil {
		return ErrNotConversationMember
	}
	if !participant.IsOwner() {
		return ErrNoPermission
	}

	target, err := uc.participantRepo.Get(ctx, conversationID, targetUserID)
	if err != nil {
		return fmt.Errorf("get target: %w", err)
	}
	if target == nil {
		return ErrNotConversationMember
	}

	target.SetRole(role)
	if err := uc.participantRepo.Update(ctx, target); err != nil {
		return fmt.Errorf("update role: %w", err)
	}

	return nil
}

func (uc *ConversationUseCaseImpl) MuteMember(ctx context.Context, operatorID, conversationID, targetUserID uint64, muteSeconds int64) error {
	operator, err := uc.participantRepo.Get(ctx, conversationID, operatorID)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}
	if operator == nil || !operator.CanManageMembers() {
		return ErrNoPermission
	}

	target, err := uc.participantRepo.Get(ctx, conversationID, targetUserID)
	if err != nil {
		return fmt.Errorf("get target: %w", err)
	}
	if target == nil {
		return ErrNotConversationMember
	}

	var mutedUntil *time.Time
	if muteSeconds > 0 {
		t := time.Now().Add(time.Duration(muteSeconds) * time.Second)
		mutedUntil = &t
	}
	target.Mute(mutedUntil)

	if err := uc.participantRepo.Update(ctx, target); err != nil {
		return fmt.Errorf("mute member: %w", err)
	}

	return nil
}

func (uc *ConversationUseCaseImpl) UnmuteMember(ctx context.Context, operatorID, conversationID, targetUserID uint64) error {
	operator, err := uc.participantRepo.Get(ctx, conversationID, operatorID)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}
	if operator == nil || !operator.CanManageMembers() {
		return ErrNoPermission
	}

	target, err := uc.participantRepo.Get(ctx, conversationID, targetUserID)
	if err != nil {
		return fmt.Errorf("get target: %w", err)
	}
	if target == nil {
		return ErrNotConversationMember
	}

	target.Unmute()
	if err := uc.participantRepo.Update(ctx, target); err != nil {
		return fmt.Errorf("unmute member: %w", err)
	}

	return nil
}
