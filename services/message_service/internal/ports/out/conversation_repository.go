package out

import "context"

// ConversationMemberRepository 提供会话成员读取能力
type ConversationMemberRepository interface {
	ListMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error)
}
