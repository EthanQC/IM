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
	ErrSelfContact        = errors.New("cannot add yourself as contact")
	ErrTargetNotFound     = errors.New("target user not found")
	ErrAlreadyContact     = errors.New("already contact")
	ErrContactBlocked     = errors.New("contact is blocked")
	ErrApplyAlreadyExists = errors.New("contact apply already exists")
	ErrApplyNotFound      = errors.New("contact apply not found")
)

type ContactUseCaseImpl struct {
	contactRepo   out.ContactRepository
	applyRepo     out.ContactApplyRepository
	blacklistRepo out.BlacklistRepository
	userRepo      out.UserRepository
}

var _ in.ContactUseCase = (*ContactUseCaseImpl)(nil)

func NewContactUseCaseImpl(
	contactRepo out.ContactRepository,
	applyRepo out.ContactApplyRepository,
	blacklistRepo out.BlacklistRepository,
	userRepo out.UserRepository,
) *ContactUseCaseImpl {
	return &ContactUseCaseImpl{
		contactRepo:   contactRepo,
		applyRepo:     applyRepo,
		blacklistRepo: blacklistRepo,
		userRepo:      userRepo,
	}
}

func (uc *ContactUseCaseImpl) ApplyContact(ctx context.Context, fromUserID, toUserID uint64, message *string) error {
	if fromUserID == toUserID {
		return ErrSelfContact
	}

	target, err := uc.userRepo.GetByID(ctx, toUserID)
	if err != nil {
		return fmt.Errorf("get target user: %w", err)
	}
	if target == nil {
		return ErrTargetNotFound
	}

	if uc.blacklistRepo != nil {
		blocked, err := uc.blacklistRepo.IsBlocked(ctx, toUserID, fromUserID)
		if err != nil {
			return fmt.Errorf("check blocked: %w", err)
		}
		if blocked {
			return ErrContactBlocked
		}
		blocked, err = uc.blacklistRepo.IsBlocked(ctx, fromUserID, toUserID)
		if err != nil {
			return fmt.Errorf("check blocked: %w", err)
		}
		if blocked {
			return ErrContactBlocked
		}
	}

	exists, err := uc.contactRepo.GetContact(ctx, fromUserID, toUserID)
	if err != nil {
		return fmt.Errorf("get contact: %w", err)
	}
	if exists != nil && exists.IsActive() {
		return ErrAlreadyContact
	}

	pending, err := uc.applyRepo.GetPendingApply(ctx, fromUserID, toUserID)
	if err != nil {
		return fmt.Errorf("get pending apply: %w", err)
	}
	if pending != nil {
		// 相同方向重复申请按幂等处理，避免前端重复提交导致报错。
		return nil
	}

	// 若对方已向当前用户发起申请，则自动互加联系人并将对方申请标记为已同意。
	reversePending, err := uc.applyRepo.GetPendingApply(ctx, toUserID, fromUserID)
	if err != nil {
		return fmt.Errorf("get reverse pending apply: %w", err)
	}
	if reversePending != nil {
		reversePending.Accept()
		if err := uc.applyRepo.Update(ctx, reversePending); err != nil {
			return fmt.Errorf("update reverse apply: %w", err)
		}

		if err := uc.ensureContact(ctx, fromUserID, toUserID); err != nil {
			return fmt.Errorf("create contact for sender: %w", err)
		}
		if err := uc.ensureContact(ctx, toUserID, fromUserID); err != nil {
			return fmt.Errorf("create contact for receiver: %w", err)
		}
		return nil
	}

	apply := &entity.ContactApply{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Message:    message,
		Status:     entity.ApplyStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	return uc.applyRepo.Create(ctx, apply)
}

func (uc *ContactUseCaseImpl) RespondContact(ctx context.Context, fromUserID uint64, userID uint64, accept bool) error {
	apply, err := uc.applyRepo.GetPendingApply(ctx, fromUserID, userID)
	if err != nil {
		return fmt.Errorf("get pending apply: %w", err)
	}
	if apply == nil {
		return ErrApplyNotFound
	}

	if accept {
		apply.Accept()
	} else {
		apply.Reject()
	}

	if err := uc.applyRepo.Update(ctx, apply); err != nil {
		return fmt.Errorf("update apply: %w", err)
	}

	if !accept {
		return nil
	}

	if err := uc.ensureContact(ctx, userID, fromUserID); err != nil {
		return fmt.Errorf("create contact for receiver: %w", err)
	}
	if err := uc.ensureContact(ctx, fromUserID, userID); err != nil {
		return fmt.Errorf("create contact for sender: %w", err)
	}

	return nil
}

func (uc *ContactUseCaseImpl) ensureContact(ctx context.Context, userID, friendID uint64) error {
	contact, err := uc.contactRepo.GetContact(ctx, userID, friendID)
	if err != nil {
		return err
	}
	if contact == nil {
		contact = &entity.Contact{
			UserID:    userID,
			FriendID:  friendID,
			Status:    entity.ContactStatusNormal,
			Type:      entity.ContactTypeFriend,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return uc.contactRepo.Create(ctx, contact)
	}

	contact.Status = entity.ContactStatusNormal
	contact.Type = entity.ContactTypeFriend
	contact.UpdatedAt = time.Now()
	return uc.contactRepo.Update(ctx, contact)
}

func (uc *ContactUseCaseImpl) RemoveContact(ctx context.Context, userID, friendID uint64) error {
	contact, err := uc.contactRepo.GetContact(ctx, userID, friendID)
	if err != nil {
		return err
	}
	if contact != nil {
		contact.Delete()
		if err := uc.contactRepo.Update(ctx, contact); err != nil {
			return err
		}
	}

	other, err := uc.contactRepo.GetContact(ctx, friendID, userID)
	if err != nil {
		return err
	}
	if other != nil {
		other.Delete()
		if err := uc.contactRepo.Update(ctx, other); err != nil {
			return err
		}
	}

	return nil
}

func (uc *ContactUseCaseImpl) ListContacts(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Contact, int, error) {
	return uc.contactRepo.List(ctx, userID, entity.ContactStatusNormal, page, pageSize)
}

func (uc *ContactUseCaseImpl) AddToBlacklist(ctx context.Context, userID, blockedUserID uint64) error {
	if userID == blockedUserID {
		return ErrSelfContact
	}

	if uc.blacklistRepo != nil {
		if err := uc.blacklistRepo.Add(ctx, userID, blockedUserID); err != nil {
			return err
		}
	}

	contact, err := uc.contactRepo.GetContact(ctx, userID, blockedUserID)
	if err != nil {
		return err
	}
	if contact == nil {
		contact = &entity.Contact{
			UserID:    userID,
			FriendID:  blockedUserID,
			Status:    entity.ContactStatusBlocked,
			Type:      entity.ContactTypeStranger,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return uc.contactRepo.Create(ctx, contact)
	}

	contact.Block()
	return uc.contactRepo.Update(ctx, contact)
}

func (uc *ContactUseCaseImpl) RemoveFromBlacklist(ctx context.Context, userID, blockedUserID uint64) error {
	if uc.blacklistRepo != nil {
		if err := uc.blacklistRepo.Remove(ctx, userID, blockedUserID); err != nil {
			return err
		}
	}

	contact, err := uc.contactRepo.GetContact(ctx, userID, blockedUserID)
	if err != nil {
		return err
	}
	if contact != nil && contact.Status == entity.ContactStatusBlocked {
		contact.Unblock()
		return uc.contactRepo.Update(ctx, contact)
	}

	return nil
}
