package vo

import (
	"regexp" // 正则表达式（regular expression）标准包

	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
)

type Phone struct {
	Number string
	IP     string
}

func NewPhone(number string) (*Phone, error) {
	if !isValidPhoneNumber(number) {
		return nil, errors.ErrInvalidPhone
	}

	return &Phone{Number: number}, nil
}

// 在程序启动时依次完成，不会在每次校验时重复编译
// 出错更早，正则表达式如果写错，程序启动就会 panic，避免到运行时才发现
// 正则表达式规则：（大陆手机号规则）
// ^ 和 $ 分别是 “从开头” 与 “到结尾” 的锚点，保证整个字符串完全符合规则
// 1 表示第一位必须是数字 1，[3-9] 表示第二位在三到九之间
// \d{9} 表示后面再跟恰好九个数字，\d 表示任意数字，{9} 表示重复九次
var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

// 校验手机号是否有效：11 位
func isValidPhoneNumber(number string) bool {
	return phoneRegex.MatchString(number)
}
