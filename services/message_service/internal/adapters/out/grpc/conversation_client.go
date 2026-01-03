package grpc

import (
	"context"
	"time"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

// ConversationClient gRPC会话服务适配器
type ConversationClient struct {
	client  imv1.ConversationServiceClient
	timeout time.Duration
}

func NewConversationClient(client imv1.ConversationServiceClient, timeout time.Duration) out.ConversationMemberRepository {
	return &ConversationClient{client: client, timeout: timeout}
}

func (c *ConversationClient) ListMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetMembers(ctx, &imv1.GetMembersRequest{ConversationId: int64(conversationID)})
	if err != nil {
		return nil, err
	}

	memberIDs := make([]uint64, 0, len(resp.Members))
	for _, member := range resp.Members {
		if member == nil || member.Id <= 0 {
			continue
		}
		memberIDs = append(memberIDs, uint64(member.Id))
	}
	return memberIDs, nil
}
