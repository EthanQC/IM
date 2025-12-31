package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/conversation_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/conversation_service/internal/ports/out"
)

// ConversationModel GORM模型
type ConversationModel struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Type        int8      `gorm:"column:type;not null"`
	Title       *string   `gorm:"column:title;type:varchar(128)"`
	AvatarURL   *string   `gorm:"column:avatar_url;type:varchar(512)"`
	OwnerID     *uint64   `gorm:"column:owner_id"`
	MemberLimit int       `gorm:"column:member_limit;default:500"`
	JoinMode    int8      `gorm:"column:join_mode;default:0"`
	MuteAll     int8      `gorm:"column:mute_all;default:0"`
	Status      int8      `gorm:"column:status;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ConversationModel) TableName() string {
	return "conversations"
}

func (m *ConversationModel) toEntity() *entity.Conversation {
	return &entity.Conversation{
		ID:          m.ID,
		Type:        entity.ConversationType(m.Type),
		Title:       m.Title,
		AvatarURL:   m.AvatarURL,
		OwnerID:     m.OwnerID,
		MemberLimit: m.MemberLimit,
		JoinMode:    entity.JoinMode(m.JoinMode),
		MuteAll:     m.MuteAll == 1,
		Status:      entity.ConversationStatus(m.Status),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func conversationModelFromEntity(e *entity.Conversation) *ConversationModel {
	muteAll := int8(0)
	if e.MuteAll {
		muteAll = 1
	}
	return &ConversationModel{
		ID:          e.ID,
		Type:        int8(e.Type),
		Title:       e.Title,
		AvatarURL:   e.AvatarURL,
		OwnerID:     e.OwnerID,
		MemberLimit: e.MemberLimit,
		JoinMode:    int8(e.JoinMode),
		MuteAll:     muteAll,
		Status:      int8(e.Status),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// ConversationRepositoryMySQL MySQL会话仓储实现
type ConversationRepositoryMySQL struct {
	db *gorm.DB
}

func NewConversationRepositoryMySQL(db *gorm.DB) out.ConversationRepository {
	return &ConversationRepositoryMySQL{db: db}
}

func (r *ConversationRepositoryMySQL) Create(ctx context.Context, conv *entity.Conversation) error {
	model := conversationModelFromEntity(conv)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	conv.ID = model.ID
	conv.CreatedAt = model.CreatedAt
	conv.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *ConversationRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.Conversation, error) {
	var model ConversationModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ConversationRepositoryMySQL) Update(ctx context.Context, conv *entity.Conversation) error {
	model := conversationModelFromEntity(conv)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *ConversationRepositoryMySQL) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&ConversationModel{}, id).Error
}

func (r *ConversationRepositoryMySQL) GetSingleConversation(ctx context.Context, userID1, userID2 uint64) (*entity.Conversation, error) {
	// 查找同时包含两个用户的单聊会话
	var model ConversationModel
	err := r.db.WithContext(ctx).
		Table("conversations c").
		Joins("JOIN participants p1 ON c.id = p1.conversation_id AND p1.user_id = ?", userID1).
		Joins("JOIN participants p2 ON c.id = p2.conversation_id AND p2.user_id = ?", userID2).
		Where("c.type = ? AND c.status = ?", entity.ConversationTypeSingle, entity.ConversationStatusNormal).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ConversationRepositoryMySQL) ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Conversation, int, error) {
	var models []ConversationModel
	var total int64

	subQuery := r.db.Model(&ParticipantModel{}).
		Select("conversation_id").
		Where("user_id = ?", userID)

	query := r.db.WithContext(ctx).Model(&ConversationModel{}).
		Where("id IN (?) AND status = ?", subQuery, entity.ConversationStatusNormal)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("updated_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	conversations := make([]*entity.Conversation, len(models))
	for i, m := range models {
		conversations[i] = m.toEntity()
	}

	return conversations, int(total), nil
}

// ParticipantModel GORM模型
type ParticipantModel struct {
	ID             uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	ConversationID uint64     `gorm:"column:conversation_id;not null;index"`
	UserID         uint64     `gorm:"column:user_id;not null;index"`
	Role           int8       `gorm:"column:role;default:0"`
	Nickname       *string    `gorm:"column:nickname;type:varchar(64)"`
	Muted          int8       `gorm:"column:muted;default:0"`
	MutedUntil     *time.Time `gorm:"column:muted_until"`
	JoinedAt       time.Time  `gorm:"column:joined_at;autoCreateTime"`
	LastReadSeq    uint64     `gorm:"column:last_read_seq;default:0"`
}

func (ParticipantModel) TableName() string {
	return "participants"
}

func (m *ParticipantModel) toEntity() *entity.Participant {
	return &entity.Participant{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		UserID:         m.UserID,
		Role:           entity.ParticipantRole(m.Role),
		Nickname:       m.Nickname,
		Muted:          m.Muted == 1,
		MutedUntil:     m.MutedUntil,
		JoinedAt:       m.JoinedAt,
		LastReadSeq:    m.LastReadSeq,
	}
}

func participantModelFromEntity(e *entity.Participant) *ParticipantModel {
	muted := int8(0)
	if e.Muted {
		muted = 1
	}
	return &ParticipantModel{
		ID:             e.ID,
		ConversationID: e.ConversationID,
		UserID:         e.UserID,
		Role:           int8(e.Role),
		Nickname:       e.Nickname,
		Muted:          muted,
		MutedUntil:     e.MutedUntil,
		JoinedAt:       e.JoinedAt,
		LastReadSeq:    e.LastReadSeq,
	}
}

// ParticipantRepositoryMySQL MySQL成员仓储实现
type ParticipantRepositoryMySQL struct {
	db *gorm.DB
}

func NewParticipantRepositoryMySQL(db *gorm.DB) out.ParticipantRepository {
	return &ParticipantRepositoryMySQL{db: db}
}

func (r *ParticipantRepositoryMySQL) Create(ctx context.Context, p *entity.Participant) error {
	model := participantModelFromEntity(p)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	p.ID = model.ID
	p.JoinedAt = model.JoinedAt
	return nil
}

func (r *ParticipantRepositoryMySQL) CreateBatch(ctx context.Context, participants []*entity.Participant) error {
	if len(participants) == 0 {
		return nil
	}
	models := make([]*ParticipantModel, len(participants))
	for i, p := range participants {
		models[i] = participantModelFromEntity(p)
	}
	return r.db.WithContext(ctx).Create(models).Error
}

func (r *ParticipantRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.Participant, error) {
	var model ParticipantModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ParticipantRepositoryMySQL) Get(ctx context.Context, conversationID, userID uint64) (*entity.Participant, error) {
	var model ParticipantModel
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ParticipantRepositoryMySQL) List(ctx context.Context, conversationID uint64) ([]*entity.Participant, error) {
	var models []ParticipantModel
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("role DESC, joined_at ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	participants := make([]*entity.Participant, len(models))
	for i, m := range models {
		participants[i] = m.toEntity()
	}
	return participants, nil
}

func (r *ParticipantRepositoryMySQL) ListByUserID(ctx context.Context, userID uint64) ([]uint64, error) {
	var convIDs []uint64
	err := r.db.WithContext(ctx).
		Model(&ParticipantModel{}).
		Where("user_id = ?", userID).
		Pluck("conversation_id", &convIDs).Error
	return convIDs, err
}

func (r *ParticipantRepositoryMySQL) Update(ctx context.Context, p *entity.Participant) error {
	model := participantModelFromEntity(p)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *ParticipantRepositoryMySQL) Delete(ctx context.Context, conversationID, userID uint64) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Delete(&ParticipantModel{}).Error
}

func (r *ParticipantRepositoryMySQL) DeleteBatch(ctx context.Context, conversationID uint64, userIDs []uint64) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id IN ?", conversationID, userIDs).
		Delete(&ParticipantModel{}).Error
}

func (r *ParticipantRepositoryMySQL) Count(ctx context.Context, conversationID uint64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&ParticipantModel{}).
		Where("conversation_id = ?", conversationID).
		Count(&count).Error
	return int(count), err
}

func (r *ParticipantRepositoryMySQL) IsMember(ctx context.Context, conversationID, userID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&ParticipantModel{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error
	return count > 0, err
}
