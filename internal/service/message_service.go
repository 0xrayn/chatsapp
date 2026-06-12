package service

import (
	"errors"

	"chatapp/internal/domain"

	"github.com/google/uuid"
)

type MessageService struct {
	msgRepo  domain.MessageRepository
	roomRepo domain.RoomRepository
}

func NewMessageService(msgRepo domain.MessageRepository, roomRepo domain.RoomRepository) *MessageService {
	return &MessageService{msgRepo: msgRepo, roomRepo: roomRepo}
}

func (s *MessageService) GetMessages(roomID, userID uuid.UUID, page, limit int) (*domain.PaginatedResponse, error) {
	// Verify user is a member
	isMember, _ := s.roomRepo.IsMember(roomID, userID)
	if !isMember {
		return nil, errors.New("you are not a member of this room")
	}

	messages, total, err := s.msgRepo.FindByRoomID(roomID, page, limit)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}

	return &domain.PaginatedResponse{
		Data:       messages,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *MessageService) EditMessage(messageID, userID uuid.UUID, req domain.EditMessageRequest) (*domain.Message, error) {
	message, err := s.msgRepo.FindByID(messageID)
	if err != nil {
		return nil, errors.New("message not found")
	}

	if message.SenderID != userID {
		return nil, errors.New("you can only edit your own messages")
	}

	if message.IsDeleted {
		return nil, errors.New("cannot edit a deleted message")
	}

	message.Content = req.Content
	message.IsEdited = true

	if err := s.msgRepo.Update(message); err != nil {
		return nil, errors.New("failed to update message")
	}

	return message, nil
}

func (s *MessageService) DeleteMessage(messageID, userID uuid.UUID) error {
	message, err := s.msgRepo.FindByID(messageID)
	if err != nil {
		return errors.New("message not found")
	}

	if message.SenderID != userID {
		return errors.New("you can only delete your own messages")
	}

	return s.msgRepo.SoftDelete(messageID)
}
