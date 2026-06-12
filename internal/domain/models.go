package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	Username  string     `json:"username" gorm:"uniqueIndex;not null"`
	Email     string     `json:"email" gorm:"uniqueIndex;not null"`
	Password  string     `json:"-" gorm:"not null"`
	Avatar    string     `json:"avatar"`
	IsOnline  bool       `json:"is_online" gorm:"default:false"`
	LastSeen  time.Time  `json:"last_seen"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Room struct {
	ID          uuid.UUID     `json:"id" gorm:"type:uuid;primary_key"`
	Name        string        `json:"name" gorm:"not null"`
	Description string        `json:"description"`
	Type        RoomType      `json:"type" gorm:"default:'public'"`
	CreatedBy   uuid.UUID     `json:"created_by" gorm:"type:uuid"`
	Creator     User          `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Members     []RoomMember  `json:"members,omitempty"`
	Messages    []Message     `json:"messages,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type RoomType string

const (
	RoomTypePublic  RoomType = "public"
	RoomTypePrivate RoomType = "private"
	RoomTypeDirect  RoomType = "direct"
)

type RoomMember struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	RoomID   uuid.UUID `json:"room_id" gorm:"type:uuid;not null"`
	UserID   uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	User     User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Role     MemberRole `json:"role" gorm:"default:'member'"`
	JoinedAt time.Time `json:"joined_at"`
}

type MemberRole string

const (
	MemberRoleAdmin  MemberRole = "admin"
	MemberRoleMember MemberRole = "member"
)

type Message struct {
	ID         uuid.UUID   `json:"id" gorm:"type:uuid;primary_key"`
	RoomID     uuid.UUID   `json:"room_id" gorm:"type:uuid;not null"`
	SenderID   uuid.UUID   `json:"sender_id" gorm:"type:uuid;not null"`
	Sender     User        `json:"sender,omitempty" gorm:"foreignKey:SenderID"`
	Content    string      `json:"content" gorm:"not null"`
	Type       MessageType `json:"type" gorm:"default:'text'"`
	IsEdited   bool        `json:"is_edited" gorm:"default:false"`
	IsDeleted  bool        `json:"is_deleted" gorm:"default:false"`
	ReplyToID  *uuid.UUID  `json:"reply_to_id,omitempty" gorm:"type:uuid"`
	ReplyTo    *Message    `json:"reply_to,omitempty" gorm:"foreignKey:ReplyToID"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeSystem MessageType = "system"
)

// WebSocket event structs
type WSEvent struct {
	Type    WSEventType `json:"type"`
	Payload interface{} `json:"payload"`
}

type WSEventType string

const (
	EventNewMessage     WSEventType = "new_message"
	EventEditMessage    WSEventType = "edit_message"
	EventDeleteMessage  WSEventType = "delete_message"
	EventUserJoined     WSEventType = "user_joined"
	EventUserLeft       WSEventType = "user_left"
	EventUserOnline     WSEventType = "user_online"
	EventUserOffline    WSEventType = "user_offline"
	EventTyping         WSEventType = "typing"
	EventStopTyping     WSEventType = "stop_typing"
	EventError          WSEventType = "error"
)

type TypingEvent struct {
	RoomID   string `json:"room_id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// Request/Response DTOs
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateRoomRequest struct {
	Name        string   `json:"name" binding:"required,min=3,max=50"`
	Description string   `json:"description"`
	Type        RoomType `json:"type"`
}

type SendMessageRequest struct {
	Content   string     `json:"content" binding:"required"`
	Type      MessageType `json:"type"`
	ReplyToID *string    `json:"reply_to_id,omitempty"`
}

type EditMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type PaginationQuery struct {
	Page  int `form:"page,default=1"`
	Limit int `form:"limit,default=20"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// DMRoom wraps a direct-message Room with the other participant's info
type DMRoom struct {
	Room      Room `json:"room"`
	OtherUser User `json:"other_user"`
}

type CreateDMRequest struct {
	RecipientID string `json:"recipient_id" binding:"required"`
}
