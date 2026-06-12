package service

import (
	"errors"
	"time"

	"chatapp/internal/domain"

	"github.com/google/uuid"
)

type RoomService struct {
	roomRepo domain.RoomRepository
}

func NewRoomService(roomRepo domain.RoomRepository) *RoomService {
	return &RoomService{roomRepo: roomRepo}
}

func (s *RoomService) CreateRoom(req domain.CreateRoomRequest, creatorID uuid.UUID) (*domain.Room, error) {
	if req.Type == "" {
		req.Type = domain.RoomTypePublic
	}

	room := &domain.Room{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		CreatedBy:   creatorID,
	}

	if err := s.roomRepo.Create(room); err != nil {
		return nil, errors.New("failed to create room")
	}

	// Add creator as admin member
	member := &domain.RoomMember{
		ID:       uuid.New(),
		RoomID:   room.ID,
		UserID:   creatorID,
		Role:     domain.MemberRoleAdmin,
		JoinedAt: time.Now(),
	}

	if err := s.roomRepo.AddMember(member); err != nil {
		return nil, errors.New("failed to add creator as member")
	}

	// Reload with relations
	return s.roomRepo.FindByID(room.ID)
}

func (s *RoomService) GetRooms(page, limit int) (*domain.PaginatedResponse, error) {
	rooms, total, err := s.roomRepo.FindAll(page, limit)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}

	return &domain.PaginatedResponse{
		Data:       rooms,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *RoomService) GetRoom(id uuid.UUID) (*domain.Room, error) {
	return s.roomRepo.FindByID(id)
}

func (s *RoomService) GetMyRooms(userID uuid.UUID) ([]domain.Room, error) {
	return s.roomRepo.FindByUserID(userID)
}

func (s *RoomService) JoinRoom(roomID, userID uuid.UUID) error {
	room, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	if room.Type == domain.RoomTypePrivate {
		return errors.New("this is a private room, you need an invitation")
	}

	isMember, _ := s.roomRepo.IsMember(roomID, userID)
	if isMember {
		return errors.New("already a member")
	}

	member := &domain.RoomMember{
		ID:       uuid.New(),
		RoomID:   roomID,
		UserID:   userID,
		Role:     domain.MemberRoleMember,
		JoinedAt: time.Now(),
	}

	return s.roomRepo.AddMember(member)
}

func (s *RoomService) LeaveRoom(roomID, userID uuid.UUID) error {
	isMember, _ := s.roomRepo.IsMember(roomID, userID)
	if !isMember {
		return errors.New("not a member of this room")
	}

	return s.roomRepo.RemoveMember(roomID, userID)
}

func (s *RoomService) DeleteRoom(roomID, userID uuid.UUID) error {
	room, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	if room.CreatedBy != userID {
		return errors.New("only room creator can delete this room")
	}

	return s.roomRepo.Delete(roomID)
}
