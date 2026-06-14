package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"chatapp/internal/domain"
	ws "chatapp/internal/websocket"

	"github.com/google/uuid"
)

type DMService struct {
	roomRepo domain.RoomRepository
	userRepo domain.UserRepository
	msgRepo  domain.MessageRepository
	hub      *ws.Hub
}

func NewDMService(roomRepo domain.RoomRepository, userRepo domain.UserRepository, msgRepo domain.MessageRepository, hub *ws.Hub) *DMService {
	return &DMService{roomRepo: roomRepo, userRepo: userRepo, msgRepo: msgRepo, hub: hub}
}

// GetAllDMPartnerIDs returns the user IDs of everyone this user has a DM room with,
// regardless of whether any messages have been exchanged yet.
func (s *DMService) GetAllDMPartnerIDs(userID uuid.UUID) ([]uuid.UUID, error) {
	rooms, err := s.roomRepo.FindDirectRoomsByUserID(userID)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(rooms))
	for _, room := range rooms {
		for _, m := range room.Members {
			if m.UserID != userID {
				ids = append(ids, m.UserID)
			}
		}
	}
	return ids, nil
}

// GetRoomMemberIDs returns the user IDs of all members in a room
func (s *DMService) GetRoomMemberIDs(roomID uuid.UUID) ([]uuid.UUID, error) {
	members, err := s.roomRepo.GetMembers(roomID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	return ids, nil
}

// HasDirectRoom checks if a DM room already exists between the two users
func (s *DMService) HasDirectRoom(userAID, userBID uuid.UUID) (bool, error) {
	room, err := s.roomRepo.FindDirectRoom(userAID, userBID)
	if err != nil || room == nil {
		return false, nil
	}
	return true, nil
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

// GetDMRooms returns all DM rooms for a user, enriched with last message,
// unread count, sorted by most recent activity first.
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

		dm := domain.DMRoom{
			Room:      room,
			OtherUser: *otherUser,
		}

		// Use live WebSocket connection status rather than the (possibly stale) DB flag
		dm.OtherUser.IsOnline = s.hub.IsUserOnline(otherUser.ID)

		// Last message preview
		if lastMsg, err := s.msgRepo.GetLastMessage(room.ID); err == nil {
			dm.LastMessage = lastMsg
		} else {
			// Skip empty conversations (no messages yet) from the sidebar list —
			// they still exist so the user can open and start chatting,
			// but shouldn't clutter the list until there's an actual message.
			continue
		}

		// Unread count: messages sent by the other user after this user's last read time
		readAt, _ := s.roomRepo.GetReadAt(room.ID, userID)
		if count, err := s.msgRepo.CountUnread(room.ID, userID, readAt); err == nil {
			dm.UnreadCount = count
		}

		dmRooms = append(dmRooms, dm)
	}

	// Sort by last message time (most recent first), fallback to room.UpdatedAt
	sort.Slice(dmRooms, func(i, j int) bool {
		ti := dmRooms[i].Room.UpdatedAt
		if dmRooms[i].LastMessage != nil {
			ti = dmRooms[i].LastMessage.CreatedAt
		}
		tj := dmRooms[j].Room.UpdatedAt
		if dmRooms[j].LastMessage != nil {
			tj = dmRooms[j].LastMessage.CreatedAt
		}
		return ti.After(tj)
	})

	return dmRooms, nil
}

// MarkAsRead marks all messages in a room as read for this user
func (s *DMService) MarkAsRead(roomID, userID uuid.UUID) error {
	isMember, _ := s.roomRepo.IsMember(roomID, userID)
	if !isMember {
		return errors.New("not a member of this room")
	}
	return s.roomRepo.MarkAsRead(roomID, userID)
}
