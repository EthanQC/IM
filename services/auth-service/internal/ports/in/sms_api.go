package in

type SmsApi interface {
	SendAuthCode(phone string, ip string) error
	VerifyAuthCode(phone string, code string) error
}
