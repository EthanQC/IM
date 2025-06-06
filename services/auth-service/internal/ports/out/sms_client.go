package out

import "context"

type SMSClient interface {
	Send(ctx context.Context, phone string, code string) error
}
