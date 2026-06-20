package repository

import (
	"time"

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

	// Include soft-deleted messages: the client renders them as a
	// "This message was deleted" placeholder so both participants see a
	// consistent view, instead of the message silently vanishing for
	// whichever side reloads history after the other side deleted it.
	r.db.Model(&domain.Message{}).
		Where("room_id = ?", roomID).
		Count(&total)

	err := r.db.
		Preload("Sender").
		Preload("ReplyTo.Sender").
		Where("room_id = ?", roomID).
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
			"content":    "",
			"type":       domain.MessageTypeText,
			"file_url":   "",
			"file_name":  "",
			"file_size":  0,
		}).Error
}

func (r *messageRepository) GetLastMessage(roomID uuid.UUID) (*domain.Message, error) {
	var message domain.Message
	// Include soft-deleted messages: if the most recent message was deleted,
	// the sidebar should show "Message deleted" rather than silently
	// falling back to an older message.
	err := r.db.
		Preload("Sender").
		Where("room_id = ?", roomID).
		Order("created_at DESC").
		First(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *messageRepository) CountUnread(roomID, userID uuid.UUID, since *time.Time) (int64, error) {
	var count int64
	q := r.db.Model(&domain.Message{}).
		Where("room_id = ? AND is_deleted = ? AND sender_id != ?", roomID, false, userID)

	if since != nil {
		q = q.Where("created_at > ?", *since)
	}

	err := q.Count(&count).Error
	return count, err
}
