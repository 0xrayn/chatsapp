package mock

import (
	"chatapp/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ---- UserRepository Mock ----

type UserRepository struct {
	mock.Mock
}

func (m *UserRepository) Create(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepository) FindByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) FindByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) FindByUsername(username string) (*domain.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) Update(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *UserRepository) SetOnlineStatus(id uuid.UUID, isOnline bool) error {
	args := m.Called(id, isOnline)
	return args.Error(0)
}

func (m *UserRepository) GetOnlineUsers() ([]domain.User, error) {
	args := m.Called()
	return args.Get(0).([]domain.User), args.Error(1)
}

func (m *UserRepository) SearchUsers(query string, excludeID uuid.UUID, limit int) ([]domain.User, error) {
	args := m.Called(query, excludeID, limit)
	return args.Get(0).([]domain.User), args.Error(1)
}

// ---- RoomRepository Mock ----

type RoomRepository struct {
	mock.Mock
}

func (m *RoomRepository) Create(room *domain.Room) error {
	args := m.Called(room)
	return args.Error(0)
}

func (m *RoomRepository) FindByID(id uuid.UUID) (*domain.Room, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *RoomRepository) FindAll(page, limit int) ([]domain.Room, int64, error) {
	args := m.Called(page, limit)
	return args.Get(0).([]domain.Room), args.Get(1).(int64), args.Error(2)
}

func (m *RoomRepository) FindByUserID(userID uuid.UUID) ([]domain.Room, error) {
	args := m.Called(userID)
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *RoomRepository) Update(room *domain.Room) error {
	args := m.Called(room)
	return args.Error(0)
}

func (m *RoomRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *RoomRepository) AddMember(member *domain.RoomMember) error {
	args := m.Called(member)
	return args.Error(0)
}

func (m *RoomRepository) RemoveMember(roomID, userID uuid.UUID) error {
	args := m.Called(roomID, userID)
	return args.Error(0)
}

func (m *RoomRepository) IsMember(roomID, userID uuid.UUID) (bool, error) {
	args := m.Called(roomID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *RoomRepository) GetMembers(roomID uuid.UUID) ([]domain.RoomMember, error) {
	args := m.Called(roomID)
	return args.Get(0).([]domain.RoomMember), args.Error(1)
}

func (m *RoomRepository) FindDirectRoom(userAID, userBID uuid.UUID) (*domain.Room, error) {
	args := m.Called(userAID, userBID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *RoomRepository) FindDirectRoomsByUserID(userID uuid.UUID) ([]domain.Room, error) {
	args := m.Called(userID)
	return args.Get(0).([]domain.Room), args.Error(1)
}

// ---- MessageRepository Mock ----

type MessageRepository struct {
	mock.Mock
}

func (m *MessageRepository) Create(message *domain.Message) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MessageRepository) FindByID(id uuid.UUID) (*domain.Message, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MessageRepository) FindByRoomID(roomID uuid.UUID, page, limit int) ([]domain.Message, int64, error) {
	args := m.Called(roomID, page, limit)
	return args.Get(0).([]domain.Message), args.Get(1).(int64), args.Error(2)
}

func (m *MessageRepository) Update(message *domain.Message) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MessageRepository) SoftDelete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}
