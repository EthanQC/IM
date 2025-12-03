package status

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

type CheckUserStatusUseCase struct {
	StatusRepo out.UserStatusRepository
}

func NewCheckUserStatusUseCase(statusRepo out.UserStatusRepository) *CheckUserStatusUseCase {
	return &CheckUserStatusUseCase{StatusRepo: statusRepo}
}

// Execute 检查用户封禁状态；若过期自动解封
func (uc *CheckUserStatusUseCase) Execute(ctx context.Context, userID string) error {
	us, err := uc.StatusRepo.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user status: %w", err)
	}
	if us != nil {
		if !us.IsActive() {
			return fmt.Errorf("user %s is blocked: %s", userID, us.BlockReason)
		}
		if us.IsBlocked {
			us.Unblock()
			if err := uc.StatusRepo.Save(ctx, us); err != nil {
				return fmt.Errorf("unblock user: %w", err)
			}
		}
	}
	return nil
}
