package vo

type Password struct {
	HashedValue string
}

func NewPassword(plaintext string) (*Password, error) {

}

func (pw *Password) Matches(plaintext string) bool {
	return
}
