package contact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

var (
	ErrContactAlreadyExists  = errors.New("contact already exists")
	ErrContactNotFound       = errors.New("contact not found")
	ErrCannotAddSelf         = errors.New("cannot add yourself as contact")
	ErrApplyAlreadyPending   = errors.New("apply already pending")
	ErrApplyNotFound         = errors.New("apply not found")
	ErrAlreadyBlocked        = errors.New("user already blocked")
	ErrNotBlocked            = errors.New("user not blocked")
	ErrUserBlocked           = errors.New("you are blocked by this user")
)

type ContactUseCaseImpl struct {
	contactRepo      out.ContactRepository
	applyRepo        out.ContactApplyRepository
	blacklistRepo    out.BlacklistRepository
	userRepo         out.UserRepository
	eventPub         out.EventPublisher
}

var _ in.ContactUseCase = (*ContactUseCaseImpl)(nil)

func NewContactUseCaseImpl(
	contactRepo out.ContactRepository,
	applyRepo out.ContactApplyRepository,
	blacklistRepo out.BlacklistRepository,
	userRepo out.UserRepository,
	eventPub out.EventPublisher,
) *ContactUseCaseImpl {
	return &ContactUseCaseImpl{
		contactRepo:   contactRepo,
		applyRepo:     applyRepo,
		blacklistRepo: blacklistRepo,
		userRepo:      userRepo,
		eventPub:      eventPub,
	}
}

func (uc *ContactUseCaseImpl) ApplyContact(ctx context.Context, fromUserID, toUserID uint64, message *string) error {
	// 不能添加自己
	if fromUserID == toUserID {
		return ErrCannotAddSelf
	}

	// 检查目标用户是否存在
	targetUser, err := uc.userRepo.GetByID(ctx, toUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}
	if targetUser == nil {
		return errors.New("target user not found")
	}

	// 检查是否被对方拉黑
	isBlocked, err := uc.blacklistRepo.IsBlocked(ctx, toUserID, fromUserID)
	if err != nil {
		return fmt.Errorf("check blacklist: %w", err)
	}
	if isBlocked {
		return ErrUserBlocked
	}

	// 检查是否已经是好友
	isContact, err := uc.contactRepo.IsContact(ctx, fromUserID, toUserID)
	if err != nil {
		return fmt.Errorf("check contact: %w", err)
	}
	if isContact {
		return ErrContactAlreadyExists
	}

	// 检查是否有待处理的申请
	pendingApply, err := uc.applyRepo.GetPendingApply(ctx, fromUserID, toUserID)
	if err != nil {
		return fmt.Errorf("get pending apply: %w", err)
	}
	if pendingApply != nil {
		return ErrApplyAlreadyPending
	}

	// 创建好友申请
	apply := &entity.ContactApply{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Message:    message,
		Status:     entity.ApplyStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := uc.applyRepo.Create(ctx, apply); err != nil {
		return fmt.Errorf("create apply: %w", err)
	}

	// 发布申请事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":         "contact.apply_created",
			"from_user_id": fromUserID,
			"to_user_id":   toUserID,
			"apply_id":     apply.ID,
			"timestamp":    apply.CreatedAt,
		}
		_ = uc.eventPub.Publish(ctx, "contact-events", event)
	}

	return nil
}

func (uc *ContactUseCaseImpl) RespondContact(ctx context.Context, applyID uint64, userID uint64, accept bool) error {
	// 获取申请
	apply, err := uc.applyRepo.GetByID(ctx, applyID)
	if err != nil {
		return fmt.Errorf("get apply: %w", err)
	}
	if apply == nil {
		return ErrApplyNotFound
	}

	// 验证是否为被申请人
	if apply.ToUserID != userID {
		return errors.New("unauthorized")
	}

	// 检查申请状态
	if !apply.IsPending() {
		return errors.New("apply already processed")
	}

	if accept {
		apply.Accept()

		// 创建双向好友关系
		now := time.Now()
		contact1 := &entity.Contact{
			UserID:    apply.FromUserID,
			FriendID:  apply.ToUserID,
			Status:    entity.ContactStatusNormal,
			Type:      entity.ContactTypeFriend,
			CreatedAt: now,
			UpdatedAt: now,
		}
		contact2 := &entity.Contact{
			UserID:    apply.ToUserID,
			FriendID:  apply.FromUserID,
			Status:    entity.ContactStatusNormal,
			Type:      entity.ContactTypeFriend,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := uc.contactRepo.Create(ctx, contact1); err != nil {
			return fmt.Errorf("create contact1: %w", err)
		}
		if err := uc.contactRepo.Create(ctx, contact2); err != nil {
			return fmt.Errorf("create contact2: %w", err)
		}
	} else {
		apply.Reject()
	}

	if err := uc.applyRepo.Update(ctx, apply); err != nil {
		return fmt.Errorf("update apply: %w", err)
	}

	// 发布响应事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":         "contact.apply_responded",
			"apply_id":     applyID,
			"from_user_id": apply.FromUserID,
			"to_user_id":   apply.ToUserID,
			"accepted":     accept,
			"timestamp":    time.Now(),
		}
		_ = uc.eventPub.Publish(ctx, "contact-events", event)
	}

	return nil
}

func (uc *ContactUseCaseImpl) RemoveContact(ctx context.Context, userID, friendID uint64) error {
	// 检查是否是好友
	isContact, err := uc.contactRepo.IsContact(ctx, userID, friendID)
	if err != nil {
		return fmt.Errorf("check contact: %w", err)
	}
	if !isContact {
		return ErrContactNotFound
	}

	// 删除双向好友关系
	if err := uc.contactRepo.Delete(ctx, userID, friendID); err != nil {
		return fmt.Errorf("delete contact1: %w", err)
	}
	if err := uc.contactRepo.Delete(ctx, friendID, userID); err != nil {
		return fmt.Errorf("delete contact2: %w", err)
	}

	// 发布删除事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":      "contact.removed",
			"user_id":   userID,
			"friend_id": friendID,
			"timestamp": time.Now(),
		}
		_ = uc.eventPub.Publish(ctx, "contact-events", event)
	}

	return nil
}

func (uc *ContactUseCaseImpl) ListContacts(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Contact, int, error) {
	return uc.contactRepo.List(ctx, userID, entity.ContactStatusNormal, page, pageSize)
}

func (uc *ContactUseCaseImpl) AddToBlacklist(ctx context.Context, userID, blockedUserID uint64) error {
	if userID == blockedUserID {
		return errors.New("cannot block yourself")
	}

	// 检查是否已在黑名单
	isBlocked, err := uc.blacklistRepo.IsBlocked(ctx, userID, blockedUserID)
	if err != nil {
		return fmt.Errorf("check blacklist: %w", err)
	}
	if isBlocked {
		return ErrAlreadyBlocked
	}

	// 添加到黑名单
	if err := uc.blacklistRepo.Add(ctx, userID, blockedUserID); err != nil {
		return fmt.Errorf("add to blacklist: %w", err)
	}

	// 如果是好友，删除好友关系
	isContact, _ := uc.contactRepo.IsContact(ctx, userID, blockedUserID)
	if isContact {
		_ = uc.contactRepo.Delete(ctx, userID, blockedUserID)
		_ = uc.contactRepo.Delete(ctx, blockedUserID, userID)
	}

	// 发布事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":            "contact.blocked",
			"user_id":         userID,
			"blocked_user_id": blockedUserID,
			"timestamp":       time.Now(),
		}
		_ = uc.eventPub.Publish(ctx, "contact-events", event)
	}

	return nil
}

func (uc *ContactUseCaseImpl) RemoveFromBlacklist(ctx context.Context, userID, blockedUserID uint64) error {
	// 检查是否在黑名单
	isBlocked, err := uc.blacklistRepo.IsBlocked(ctx, userID, blockedUserID)
	if err != nil {
		return fmt.Errorf("check blacklist: %w", err)
	}
	if !isBlocked {
		return ErrNotBlocked
	}

	// 从黑名单移除
	if err := uc.blacklistRepo.Remove(ctx, userID, blockedUserID); err != nil {
		return fmt.Errorf("remove from blacklist: %w", err)
	}

	return nil
}
