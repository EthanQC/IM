package vo

import (
	"unicode"

	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

type Password struct {
	HashedValue string
}

func NewPassword(plaintext string) (*Password, error) {
	if !isValidPassword(plaintext) {
		return nil, errors.ErrInvalidPassword
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &Password{
		HashedValue: string(hashedBytes),
	}, nil
}

// 密码是否匹配
func (pw *Password) Matches(plaintext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(pw.HashedValue), []byte(plaintext))
	return err == nil
}

// 密码规则校验
func isValidPassword(password string) bool {
	// 密码规则：6-20 位，必须包含数字和字母
	if len(password) < 6 || len(password) > 20 {
		return false
	}

	hasLetter := false
	hasDigit := false

	for _, ch := range password {
		switch {
		case unicode.IsLetter(ch):
			hasLetter = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	return hasLetter && hasDigit
}
