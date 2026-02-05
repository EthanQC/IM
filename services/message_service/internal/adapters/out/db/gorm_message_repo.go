package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

// MessageModel GORM模型
type MessageModel struct {
	ID             uint64        `gorm:"column:id;primaryKey;autoIncrement"`
	ConversationID uint64        `gorm:"column:conversation_id;not null;index"`
	SenderID       uint64        `gorm:"column:sender_id;not null;index"`
	ClientMsgID    string        `gorm:"column:client_msg_id;type:varchar(64);not null"`
	Seq            uint64        `gorm:"column:seq;not null"`
	ContentType    int8          `gorm:"column:content_type;not null"`
	Content        string        `gorm:"column:content;type:json;not null"`
	Status         int8          `gorm:"column:status;default:1"`
	ReplyToMsgID   sql.NullInt64 `gorm:"column:reply_to_msg_id"`
	CreatedAt      time.Time     `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time     `gorm:"column:updated_at;autoUpdateTime"`
}

func (MessageModel) TableName() string {
	return "messages"
}

func (m *MessageModel) toEntity() *entity.Message {
	var content entity.MessageContent
	_ = json.Unmarshal([]byte(m.Content), &content)

	var replyToMsgID *uint64
	if m.ReplyToMsgID.Valid {
		id := uint64(m.ReplyToMsgID.Int64)
		replyToMsgID = &id
	}

	return &entity.Message{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		ClientMsgID:    m.ClientMsgID,
		Seq:            m.Seq,
		ContentType:    entity.MessageContentType(m.ContentType),
		Content:        content,
		Status:         entity.MessageStatus(m.Status),
		ReplyToMsgID:   replyToMsgID,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func messageModelFromEntity(e *entity.Message) *MessageModel {
	contentBytes, _ := json.Marshal(e.Content)

	var replyToMsgID sql.NullInt64
	if e.ReplyToMsgID != nil {
		replyToMsgID = sql.NullInt64{Int64: int64(*e.ReplyToMsgID), Valid: true}
	}

	return &MessageModel{
		ID:             e.ID,
		ConversationID: e.ConversationID,
		SenderID:       e.SenderID,
		ClientMsgID:    e.ClientMsgID,
		Seq:            e.Seq,
		ContentType:    int8(e.ContentType),
		Content:        string(contentBytes),
		Status:         int8(e.Status),
		ReplyToMsgID:   replyToMsgID,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}

// MessageRepositoryMySQL MySQL消息仓储实现
type MessageRepositoryMySQL struct {
	db *gorm.DB
}

func NewMessageRepositoryMySQL(db *gorm.DB) out.MessageRepository {
	return &MessageRepositoryMySQL{db: db}
}

func (r *MessageRepositoryMySQL) Create(ctx context.Context, msg *entity.Message) error {
	model := messageModelFromEntity(msg)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	msg.ID = model.ID
	msg.CreatedAt = model.CreatedAt
	msg.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *MessageRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.Message, error) {
	var model MessageModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *MessageRepositoryMySQL) GetByClientMsgID(ctx context.Context, senderID uint64, clientMsgID string) (*entity.Message, error) {
	var model MessageModel
	err := r.db.WithContext(ctx).
		Where("sender_id = ? AND client_msg_id = ?", senderID, clientMsgID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *MessageRepositoryMySQL) GetHistoryAfter(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error) {
	var models []MessageModel
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq > ? AND status = ?", conversationID, afterSeq, entity.MessageStatusNormal).
		Order("seq ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	messages := make([]*entity.Message, len(models))
	for i, m := range models {
		messages[i] = m.toEntity()
	}
	return messages, nil
}

func (r *MessageRepositoryMySQL) GetHistoryBefore(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error) {
	var models []MessageModel
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq < ? AND status = ?", conversationID, beforeSeq, entity.MessageStatusNormal).
		Order("seq DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	// 反转顺序，使其按seq升序排列
	messages := make([]*entity.Message, len(models))
	for i, m := range models {
		messages[len(models)-1-i] = m.toEntity()
	}
	return messages, nil
}

func (r *MessageRepositoryMySQL) Update(ctx context.Context, msg *entity.Message) error {
	model := messageModelFromEntity(msg)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *MessageRepositoryMySQL) GetLatestSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	var seq uint64
	err := r.db.WithContext(ctx).
		Model(&MessageModel{}).
		Where("conversation_id = ?", conversationID).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&seq).Error
	return seq, err
}

// SequenceModel 序列号模型
type SequenceModel struct {
	ConversationID uint64    `gorm:"column:conversation_id;primaryKey"`
	NextSeq        uint64    `gorm:"column:next_seq;default:1"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (SequenceModel) TableName() string {
	return "message_sequences"
}

// SequenceRepositoryMySQL MySQL序列号仓储实现
type SequenceRepositoryMySQL struct {
	db *gorm.DB
}

func NewSequenceRepositoryMySQL(db *gorm.DB) out.SequenceRepository {
	return &SequenceRepositoryMySQL{db: db}
}

func (r *SequenceRepositoryMySQL) GetNextSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	// 使用乐观锁获取并递增序号
	var seq uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model SequenceModel
		err := tx.Where("conversation_id = ?", conversationID).First(&model).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 首次创建
				model = SequenceModel{
					ConversationID: conversationID,
					NextSeq:        2,
				}
				if err := tx.Create(&model).Error; err != nil {
					return err
				}
				seq = 1
				return nil
			}
			return err
		}

		seq = model.NextSeq
		if model.NextSeq == 1 {
			var maxSeq uint64
			if err := tx.Model(&MessageModel{}).
				Select("MAX(seq)").
				Where("conversation_id = ?", conversationID).
				Scan(&maxSeq).Error; err != nil {
				return err
			}
			if maxSeq >= seq {
				seq = maxSeq + 1
				model.NextSeq = seq + 1
				return tx.Save(&model).Error
			}
		}

		model.NextSeq++
		return tx.Save(&model).Error
	})
	return seq, err
}

func (r *SequenceRepositoryMySQL) GetCurrentSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	var model SequenceModel
	err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return model.NextSeq - 1, nil
}

// InboxModel 收件箱模型
type InboxModel struct {
	UserID           uint64    `gorm:"column:user_id;primaryKey"`
	ConversationID   uint64    `gorm:"column:conversation_id;primaryKey"`
	LastReadSeq      uint64    `gorm:"column:last_read_seq;default:0"`
	LastDeliveredSeq uint64    `gorm:"column:last_delivered_seq;default:0"`
	UnreadCount      int       `gorm:"column:unread_count;default:0"`
	IsMuted          int8      `gorm:"column:is_muted;default:0"`
	IsPinned         int8      `gorm:"column:is_pinned;default:0"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (InboxModel) TableName() string {
	return "inbox"
}

func (m *InboxModel) toDTO() *out.Inbox {
	return &out.Inbox{
		UserID:           m.UserID,
		ConversationID:   m.ConversationID,
		LastReadSeq:      m.LastReadSeq,
		LastDeliveredSeq: m.LastDeliveredSeq,
		UnreadCount:      m.UnreadCount,
		IsMuted:          m.IsMuted == 1,
		IsPinned:         m.IsPinned == 1,
	}
}

// InboxRepositoryMySQL MySQL收件箱仓储实现
type InboxRepositoryMySQL struct {
	db *gorm.DB
}

func NewInboxRepositoryMySQL(db *gorm.DB) out.InboxRepository {
	return &InboxRepositoryMySQL{db: db}
}

func (r *InboxRepositoryMySQL) GetOrCreate(ctx context.Context, userID, conversationID uint64) (*out.Inbox, error) {
	var model InboxModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model = InboxModel{
				UserID:         userID,
				ConversationID: conversationID,
			}
			if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return model.toDTO(), nil
}

func (r *InboxRepositoryMySQL) UpdateLastRead(ctx context.Context, userID, conversationID, readSeq uint64) error {
	return r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Update("last_read_seq", readSeq).Error
}

func (r *InboxRepositoryMySQL) UpdateLastDelivered(ctx context.Context, userID, conversationID, deliveredSeq uint64) error {
	return r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Update("last_delivered_seq", deliveredSeq).Error
}

// UpdateLastDeliveredForReceiver 更新投递位置并原子增加未读数（接收者调用）
// 使用单条 SQL 语句同时更新两个字段，保证原子性
func (r *InboxRepositoryMySQL) UpdateLastDeliveredForReceiver(ctx context.Context, userID, conversationID, deliveredSeq uint64) error {
	return r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Updates(map[string]interface{}{
			"last_delivered_seq": deliveredSeq,
			"unread_count":       gorm.Expr("unread_count + 1"),
		}).Error
}

func (r *InboxRepositoryMySQL) IncrUnread(ctx context.Context, userID, conversationID uint64, delta int) error {
	return r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Update("unread_count", gorm.Expr("unread_count + ?", delta)).Error
}

func (r *InboxRepositoryMySQL) ClearUnread(ctx context.Context, userID, conversationID uint64) error {
	return r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Update("unread_count", 0).Error
}

func (r *InboxRepositoryMySQL) GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error) {
	var count int
	err := r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Select("unread_count").
		Scan(&count).Error
	return count, err
}

// GetTotalUnread 获取用户总未读数
func (r *InboxRepositoryMySQL) GetTotalUnread(ctx context.Context, userID uint64) (int, error) {
	var total int
	err := r.db.WithContext(ctx).
		Model(&InboxModel{}).
		Where("user_id = ? AND is_muted = 0", userID).
		Select("COALESCE(SUM(unread_count), 0)").
		Scan(&total).Error
	return total, err
}

// GetUserInboxes 获取用户的所有收件箱
func (r *InboxRepositoryMySQL) GetUserInboxes(ctx context.Context, userID uint64) ([]*entity.Inbox, error) {
	var models []InboxModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	inboxes := make([]*entity.Inbox, len(models))
	for i, m := range models {
		inboxes[i] = &entity.Inbox{
			UserID:           m.UserID,
			ConversationID:   m.ConversationID,
			LastReadSeq:      m.LastReadSeq,
			LastDeliveredSeq: m.LastDeliveredSeq,
			UnreadCount:      int(m.UnreadCount),
			IsMuted:          m.IsMuted == 1,
			IsPinned:         m.IsPinned == 1,
		}
	}

	return inboxes, nil
}
