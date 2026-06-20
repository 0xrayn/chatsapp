package service

import (
	"errors"
	"time"

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

// EditDeleteWindow is how long after sending a message can still be edited or deleted.
const EditDeleteWindow = 3 * time.Minute

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

	if time.Since(message.CreatedAt) > EditDeleteWindow {
		return nil, errors.New("messages can only be edited within 3 minutes of sending")
	}

	// Find the recipient (the other room member) and check whether they've
	// already read past this message's timestamp — if so, editing is blocked
	// so the sender can't silently change something the recipient has seen.
	members, err := s.roomRepo.GetMembers(message.RoomID)
	if err == nil {
		for _, m := range members {
			if m.UserID == userID {
				continue
			}
			if m.ReadAt != nil && !m.ReadAt.Before(message.CreatedAt) {
				return nil, errors.New("cannot edit a message the recipient has already read")
			}
		}
	}

	message.Content = req.Content
	message.IsEdited = true

	if err := s.msgRepo.Update(message); err != nil {
		return nil, errors.New("failed to update message")
	}

	return message, nil
}

// GetMessageRoomID returns the room ID a message belongs to (used by the
// handler to broadcast a delete event after the message is removed).
func (s *MessageService) GetMessageRoomID(messageID uuid.UUID) (uuid.UUID, error) {
	message, err := s.msgRepo.FindByID(messageID)
	if err != nil {
		return uuid.Nil, errors.New("message not found")
	}
	return message.RoomID, nil
}

func (s *MessageService) DeleteMessage(messageID, userID uuid.UUID) error {
	message, err := s.msgRepo.FindByID(messageID)
	if err != nil {
		return errors.New("message not found")
	}

	if message.SenderID != userID {
		return errors.New("you can only delete your own messages")
	}

	if message.IsDeleted {
		return errors.New("message already deleted")
	}

	if time.Since(message.CreatedAt) > EditDeleteWindow {
		return errors.New("messages can only be deleted within 3 minutes of sending")
	}

	// Block deletion once the recipient has already read the message —
	// same rule as editing, so a sender can't retroactively remove
	// something the other person has already seen.
	members, err := s.roomRepo.GetMembers(message.RoomID)
	if err == nil {
		for _, m := range members {
			if m.UserID == userID {
				continue
			}
			if m.ReadAt != nil && !m.ReadAt.Before(message.CreatedAt) {
				return errors.New("cannot delete a message the recipient has already read")
			}
		}
	}

	return s.msgRepo.SoftDelete(messageID)
}
