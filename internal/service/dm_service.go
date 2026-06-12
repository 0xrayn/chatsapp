package service

import (
	"errors"
	"fmt"
	"time"

	"chatapp/internal/domain"

	"github.com/google/uuid"
)

type DMService struct {
	roomRepo domain.RoomRepository
	userRepo domain.UserRepository
}

func NewDMService(roomRepo domain.RoomRepository, userRepo domain.UserRepository) *DMService {
	return &DMService{roomRepo: roomRepo, userRepo: userRepo}
}

// GetOrCreateDM returns existing DM room or creates a new one
func (s *DMService) GetOrCreateDM(senderID, recipientID uuid.UUID) (*domain.Room, error) {
	if senderID == recipientID {
		return nil, errors.New("cannot create DM with yourself")
	}

	// Check recipient exists
	recipient, err := s.userRepo.FindByID(recipientID)
	if err != nil {
		return nil, errors.New("recipient user not found")
	}

	// Try to find existing DM room between these two users
	existing, err := s.roomRepo.FindDirectRoom(senderID, recipientID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Create new DM room
	sender, err := s.userRepo.FindByID(senderID)
	if err != nil {
		return nil, errors.New("sender user not found")
	}

	room := &domain.Room{
		ID:          uuid.New(),
		Name:        fmt.Sprintf("dm:%s-%s", sender.Username, recipient.Username),
		Description: "Direct message",
		Type:        domain.RoomTypeDirect,
		CreatedBy:   senderID,
	}

	if err := s.roomRepo.Create(room); err != nil {
		return nil, errors.New("failed to create DM room")
	}

	// Add both users as members
	members := []domain.RoomMember{
		{
			ID:       uuid.New(),
			RoomID:   room.ID,
			UserID:   senderID,
			Role:     domain.MemberRoleMember,
			JoinedAt: time.Now(),
		},
		{
			ID:       uuid.New(),
			RoomID:   room.ID,
			UserID:   recipientID,
			Role:     domain.MemberRoleMember,
			JoinedAt: time.Now(),
		},
	}

	for _, m := range members {
		member := m
		if err := s.roomRepo.AddMember(&member); err != nil {
			return nil, errors.New("failed to add members to DM room")
		}
	}

	return s.roomRepo.FindByID(room.ID)
}

// GetDMRooms returns all DM rooms for a user
func (s *DMService) GetDMRooms(userID uuid.UUID) ([]domain.DMRoom, error) {
	rooms, err := s.roomRepo.FindDirectRoomsByUserID(userID)
	if err != nil {
		return nil, err
	}

	dmRooms := make([]domain.DMRoom, 0, len(rooms))
	for _, room := range rooms {
		// Find the other participant
		var otherUser *domain.User
		for _, m := range room.Members {
			if m.UserID != userID {
				otherUser = &m.User
				break
			}
		}
		if otherUser == nil {
			continue
		}

		dmRooms = append(dmRooms, domain.DMRoom{
			Room:      room,
			OtherUser: *otherUser,
		})
	}

	return dmRooms, nil
}
