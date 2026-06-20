package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRepository interface {
	Create(user *User) error
	FindByID(id uuid.UUID) (*User, error)
	FindByEmail(email string) (*User, error)
	FindByUsername(username string) (*User, error)
	Update(user *User) error
	SetOnlineStatus(id uuid.UUID, isOnline bool) error
	GetOnlineUsers() ([]User, error)
	SearchUsers(query string, excludeID uuid.UUID, limit int) ([]User, error)
}

type RoomRepository interface {
	Create(room *Room) error
	FindByID(id uuid.UUID) (*Room, error)
	FindAll(page, limit int) ([]Room, int64, error)
	FindByUserID(userID uuid.UUID) ([]Room, error)
	Update(room *Room) error
	Delete(id uuid.UUID) error
	AddMember(member *RoomMember) error
	RemoveMember(roomID, userID uuid.UUID) error
	IsMember(roomID, userID uuid.UUID) (bool, error)
	GetMembers(roomID uuid.UUID) ([]RoomMember, error)
	FindDirectRoom(userAID, userBID uuid.UUID) (*Room, error)
	FindDirectRoomsByUserID(userID uuid.UUID) ([]Room, error)
	MarkAsRead(roomID, userID uuid.UUID) error
	GetReadAt(roomID, userID uuid.UUID) (*time.Time, error)
	TouchLastMessageAt(roomID uuid.UUID) error
}

type MessageRepository interface {
	Create(message *Message) error
	FindByID(id uuid.UUID) (*Message, error)
	FindByRoomID(roomID uuid.UUID, page, limit int) ([]Message, int64, error)
	Update(message *Message) error
	SoftDelete(id uuid.UUID) error
	GetLastMessage(roomID uuid.UUID) (*Message, error)
	CountUnread(roomID, userID uuid.UUID, since *time.Time) (int64, error)
}

type TokenBlacklistRepository interface {
	Add(jti string, expiresAt time.Time) error
	IsRevoked(jti string) (bool, error)
	DeleteExpired() error
}
