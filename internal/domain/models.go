package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	Username          string     `json:"username" gorm:"uniqueIndex;not null"`
	Email             string     `json:"email" gorm:"uniqueIndex;not null"`
	Password          string     `json:"-" gorm:"not null"`
	Avatar            string     `json:"avatar"`
	Status            string     `json:"status" gorm:"default:'Hey there! I am using ChatApp'"`
	IsOnline          bool       `json:"is_online" gorm:"default:false"`
	LastSeen          time.Time  `json:"last_seen"`
	UsernameChangedAt *time.Time `json:"-" gorm:"default:null"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Room struct {
	ID             uuid.UUID    `json:"id" gorm:"type:uuid;primary_key"`
	Name           string       `json:"name" gorm:"not null"`
	Description    string       `json:"description"`
	Type           RoomType     `json:"type" gorm:"default:'public'"`
	CreatedBy      uuid.UUID    `json:"created_by" gorm:"type:uuid"`
	Creator        User         `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Members        []RoomMember `json:"members,omitempty"`
	Messages       []Message    `json:"messages,omitempty"`
	LastMessageAt  *time.Time   `json:"last_message_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type RoomType string

const (
	RoomTypePublic  RoomType = "public"
	RoomTypePrivate RoomType = "private"
	RoomTypeDirect  RoomType = "direct"
)

type RoomMember struct {
	ID       uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	RoomID   uuid.UUID  `json:"room_id" gorm:"type:uuid;not null"`
	UserID   uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	User     User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Role     MemberRole `json:"role" gorm:"default:'member'"`
	ReadAt   *time.Time `json:"read_at,omitempty"`
	JoinedAt time.Time  `json:"joined_at"`
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
	FileURL    string      `json:"file_url,omitempty"`
	FileName   string      `json:"file_name,omitempty"`
	FileSize   int64       `json:"file_size,omitempty"`
	IsEdited   bool        `json:"is_edited" gorm:"default:false"`
	IsDeleted  bool        `json:"is_deleted" gorm:"default:false"`
	ReplyToID  *uuid.UUID  `json:"reply_to_id,omitempty" gorm:"type:uuid"`
	ReplyTo    *Message    `json:"reply_to,omitempty" gorm:"foreignKey:ReplyToID"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeImage  MessageType = "image"
	MessageTypeFile   MessageType = "file"
	MessageTypeAudio  MessageType = "audio"
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
	EventDMCreated      WSEventType = "dm_created"
	EventMessagesRead   WSEventType = "messages_read"
	EventProfileUpdated WSEventType = "profile_updated"
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
	Content   string      `json:"content"`
	Type      MessageType `json:"type"`
	ReplyToID *string     `json:"reply_to_id,omitempty"`
	FileURL   string      `json:"file_url,omitempty"`
	FileName  string      `json:"file_name,omitempty"`
	FileSize  int64       `json:"file_size,omitempty"`
}

type UpdateProfileRequest struct {
	Status string `json:"status,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

type UpdateUsernameRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
}

type UpdateEmailRequest struct {
	Email           string `json:"email" binding:"required,email"`
	CurrentPassword string `json:"current_password" binding:"required"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
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

// DMRoom wraps a direct-message Room with the other participant's info,
// plus the last message preview and unread count for the requesting user.
type DMRoom struct {
	Room        Room      `json:"room"`
	OtherUser   User      `json:"other_user"`
	LastMessage *Message  `json:"last_message,omitempty"`
	UnreadCount int64     `json:"unread_count"`
}

type MarkReadRequest struct {
	RoomID string `json:"room_id"`
}

type CreateDMRequest struct {
	RecipientID string `json:"recipient_id" binding:"required"`
}
