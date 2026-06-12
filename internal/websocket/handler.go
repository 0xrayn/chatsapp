package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev; restrict in production
	},
}

type IncomingMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ChatPayload struct {
	RoomID    string `json:"room_id"`
	Content   string `json:"content"`
	ReplyToID string `json:"reply_to_id,omitempty"`
}

type TypingPayload struct {
	RoomID string `json:"room_id"`
}

type JoinRoomPayload struct {
	RoomID string `json:"room_id"`
}

type Handler struct {
	hub     *Hub
	msgRepo domain.MessageRepository
	roomRepo domain.RoomRepository
	userRepo domain.UserRepository
}

func NewHandler(hub *Hub, msgRepo domain.MessageRepository, roomRepo domain.RoomRepository, userRepo domain.UserRepository) *Handler {
	return &Handler{
		hub:      hub,
		msgRepo:  msgRepo,
		roomRepo: roomRepo,
		userRepo: userRepo,
	}
}

func (h *Handler) HandleWS(c *gin.Context) {
	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		ID:       uuid.New(),
		UserID:   userID,
		Username: username,
		Rooms:    make(map[uuid.UUID]bool),
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h.hub,
	}

	h.hub.Register <- client

	// Update online status
	go h.userRepo.SetOnlineStatus(userID, true)

	// Notify others this user is online
	go h.broadcastUserStatus(userID, username, string(domain.EventUserOnline))

	// Start write pump in a goroutine
	go client.WritePump()

	// Send welcome event
	client.SendEvent(string(domain.EventUserOnline), gin.H{
		"user_id":  userID,
		"username": username,
		"message":  "Connected to chat server",
	})

	// Read pump (blocks until connection closes)
	h.readPump(client)

	// Cleanup on disconnect
	h.hub.Unregister <- client
	h.userRepo.SetOnlineStatus(userID, false)
	h.broadcastUserStatus(userID, username, string(domain.EventUserOffline))
}

func (h *Handler) readPump(client *Client) {
	defer client.Conn.Close()

	client.Conn.SetReadLimit(4096)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, rawMsg, err := client.Conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err, gorillaws.CloseGoingAway, gorillaws.CloseAbnormalClosure) {
				log.Printf("Unexpected close for client %s: %v", client.ID, err)
			}
			break
		}

		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg IncomingMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid message format"})
			continue
		}

		switch msg.Type {
		case "join_room":
			h.handleJoinRoom(client, msg.Payload)
		case "leave_room":
			h.handleLeaveRoom(client, msg.Payload)
		case "send_message":
			h.handleSendMessage(client, msg.Payload)
		case "typing":
			h.handleTyping(client, msg.Payload, true)
		case "stop_typing":
			h.handleTyping(client, msg.Payload, false)
		case "ping":
			client.SendEvent("pong", gin.H{"time": time.Now().Unix()})
		default:
			client.SendEvent(string(domain.EventError), gin.H{"error": "Unknown event type"})
		}
	}
}

func (h *Handler) handleJoinRoom(client *Client, payload json.RawMessage) {
	var p JoinRoomPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid payload"})
		return
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid room ID"})
		return
	}

	// Check membership
	isMember, err := h.roomRepo.IsMember(roomID, client.UserID)
	if err != nil || !isMember {
		client.SendEvent(string(domain.EventError), gin.H{"error": "You are not a member of this room"})
		return
	}

	h.hub.JoinRoom(client.ID, roomID)

	// Notify room members
	h.broadcastToRoom(roomID, client.ID, string(domain.EventUserJoined), gin.H{
		"user_id":  client.UserID,
		"username": client.Username,
		"room_id":  roomID,
	})

	client.SendEvent(string(domain.EventUserJoined), gin.H{
		"room_id": roomID,
		"message": "Joined room successfully",
	})
}

func (h *Handler) handleLeaveRoom(client *Client, payload json.RawMessage) {
	var p JoinRoomPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return
	}

	h.hub.LeaveRoom(client.ID, roomID)
	h.broadcastToRoom(roomID, client.ID, string(domain.EventUserLeft), gin.H{
		"user_id":  client.UserID,
		"username": client.Username,
		"room_id":  roomID,
	})
}

func (h *Handler) handleSendMessage(client *Client, payload json.RawMessage) {
	var p ChatPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid payload"})
		return
	}

	if p.Content == "" || len(p.Content) > 2000 {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Message content must be 1-2000 characters"})
		return
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid room ID"})
		return
	}

	// Verify membership
	isMember, _ := h.roomRepo.IsMember(roomID, client.UserID)
	if !isMember {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Not a room member"})
		return
	}

	message := &domain.Message{
		ID:       uuid.New(),
		RoomID:   roomID,
		SenderID: client.UserID,
		Content:  p.Content,
		Type:     domain.MessageTypeText,
	}

	if p.ReplyToID != "" {
		replyID, err := uuid.Parse(p.ReplyToID)
		if err == nil {
			message.ReplyToID = &replyID
		}
	}

	if err := h.msgRepo.Create(message); err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Failed to save message"})
		return
	}

	// Load sender info
	savedMsg, _ := h.msgRepo.FindByID(message.ID)
	if savedMsg == nil {
		savedMsg = message
		savedMsg.Sender = domain.User{ID: client.UserID, Username: client.Username}
	}

	h.broadcastToRoom(roomID, uuid.Nil, string(domain.EventNewMessage), savedMsg)
}

func (h *Handler) handleTyping(client *Client, payload json.RawMessage, isTyping bool) {
	var p TypingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		return
	}

	eventType := string(domain.EventTyping)
	if !isTyping {
		eventType = string(domain.EventStopTyping)
	}

	h.broadcastToRoom(roomID, client.ID, eventType, domain.TypingEvent{
		RoomID:   p.RoomID,
		UserID:   client.UserID.String(),
		Username: client.Username,
	})
}

func (h *Handler) broadcastToRoom(roomID, excludeClientID uuid.UUID, eventType string, payload interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	})
	if err != nil {
		return
	}

	h.hub.Broadcast <- &RoomMessage{
		RoomID:  roomID,
		Payload: data,
		Exclude: excludeClientID,
	}
}

func (h *Handler) broadcastUserStatus(userID uuid.UUID, username, eventType string) {
	// Broadcast to all connected clients
	data, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"payload": gin.H{
			"user_id":  userID,
			"username": username,
		},
	})

	h.hub.mu.RLock()
	defer h.hub.mu.RUnlock()

	for _, client := range h.hub.clients {
		if client.UserID == userID {
			continue
		}
		select {
		case client.Send <- data:
		default:
		}
	}
}
