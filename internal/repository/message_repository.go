package repository

import (
	"chatapp/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type messageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) domain.MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(message *domain.Message) error {
	return r.db.Create(message).Error
}

func (r *messageRepository) FindByID(id uuid.UUID) (*domain.Message, error) {
	var message domain.Message
	err := r.db.
		Preload("Sender").
		Preload("ReplyTo.Sender").
		Where("id = ?", id).
		First(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *messageRepository) FindByRoomID(roomID uuid.UUID, page, limit int) ([]domain.Message, int64, error) {
	var messages []domain.Message
	var total int64

	offset := (page - 1) * limit

	r.db.Model(&domain.Message{}).
		Where("room_id = ? AND is_deleted = ?", roomID, false).
		Count(&total)

	err := r.db.
		Preload("Sender").
		Preload("ReplyTo.Sender").
		Where("room_id = ? AND is_deleted = ?", roomID, false).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&messages).Error

	// Reverse to show oldest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, total, err
}

func (r *messageRepository) Update(message *domain.Message) error {
	return r.db.Save(message).Error
}

func (r *messageRepository) SoftDelete(id uuid.UUID) error {
	return r.db.Model(&domain.Message{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_deleted": true,
			"content":    "This message has been deleted",
		}).Error
}
