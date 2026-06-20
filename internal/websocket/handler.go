package websocket

import (
	"encoding/json"
	"os"
	"strings"
	"log"
	"net/http"
	"time"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
)

// wsAllowedOrigins mirrors the ALLOWED_ORIGINS env used by the HTTP CORS middleware.
func wsAllowedOrigins() map[string]bool {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		raw = "http://localhost:8080,http://localhost:3000"
	}
	origins := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = true
		}
	}
	return origins
}

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Only allow origins listed in ALLOWED_ORIGINS env variable.
	// Prevents cross-site WebSocket hijacking from untrusted domains.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Allow same-origin requests (no Origin header, e.g. server-to-server)
		if origin == "" {
			return true
		}
		return wsAllowedOrigins()[origin]
	},
}

type IncomingMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type AuthPayload struct {
	Token string `json:"token"`
}

type ChatPayload struct {
	RoomID    string `json:"room_id"`
	Content   string `json:"content"`
	ReplyToID string `json:"reply_to_id,omitempty"`
	Type      string `json:"type,omitempty"`
	FileURL   string `json:"file_url,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
}

type TypingPayload struct {
	RoomID string `json:"room_id"`
}

type JoinRoomPayload struct {
	RoomID string `json:"room_id"`
}

type Handler struct {
	hub      *Hub
	msgRepo  domain.MessageRepository
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
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// --- Token authentication ---
	// We do NOT read the token from the URL (?token=...) because query params
	// are written to server logs, browser history, and Referer headers.
	//
	// Two supported paths:
	//   1. Authorization header was already validated by WSAuthMiddleware (non-browser clients).
	//   2. ws_pending_auth flag is set → client must send {"type":"auth","payload":{"token":"<jwt>"}}
	//      as the very first WebSocket message before anything else is processed.

	var userID uuid.UUID
	var username string

	pendingAuth, _ := c.Get("ws_pending_auth")
	if pendingAuth == true {
		// Ask the client to authenticate via first message
		authRequiredMsg, _ := json.Marshal(map[string]string{"type": "auth_required"})
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		conn.WriteMessage(gorillaws.TextMessage, authRequiredMsg)

		// Wait for the auth message (10 s timeout)
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return
		}

		var msg IncomingMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil || msg.Type != "auth" {
			errMsg, _ := json.Marshal(map[string]interface{}{"type": "error", "payload": map[string]string{"error": "First message must be auth"}})
			conn.WriteMessage(gorillaws.TextMessage, errMsg)
			conn.Close()
			return
		}

		var ap AuthPayload
		if err := json.Unmarshal(msg.Payload, &ap); err != nil || ap.Token == "" {
			errMsg, _ := json.Marshal(map[string]interface{}{"type": "error", "payload": map[string]string{"error": "Invalid auth payload"}})
			conn.WriteMessage(gorillaws.TextMessage, errMsg)
			conn.Close()
			return
		}

		claims, err := middleware.ParseToken(ap.Token)
		if err != nil {
			errMsg, _ := json.Marshal(map[string]interface{}{"type": "error", "payload": map[string]string{"error": "Invalid or expired token"}})
			conn.WriteMessage(gorillaws.TextMessage, errMsg)
			conn.Close()
			return
		}

		userID = claims.UserID
		username = claims.Username
	} else {
		// Token was validated by WSAuthMiddleware via Authorization header
		uid, exists := c.Get("user_id")
		if !exists {
			conn.Close()
			return
		}
		userID = uid.(uuid.UUID)
		username = c.MustGet("username").(string)
	}

	// Reset deadlines after successful auth
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	client := &Client{
		ID:       uuid.New(),
		UserID:   userID,
		Username: username,
		Rooms:    make(map[uuid.UUID]bool),
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h.hub,
	}

	h.hub.RegisterClient(client)

	// Auto-join every room this user belongs to
	if rooms, err := h.roomRepo.FindByUserID(userID); err == nil {
		for _, room := range rooms {
			h.hub.JoinRoom(client.ID, room.ID)
		}
	}

	go h.userRepo.SetOnlineStatus(userID, true)
	go h.broadcastUserStatus(userID, username, string(domain.EventUserOnline))
	go client.WritePump()

	client.SendEvent(string(domain.EventUserOnline), gin.H{
		"user_id":  userID,
		"username": username,
		"message":  "Connected to chat server",
	})

	h.readPump(client)

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

	isMember, err := h.roomRepo.IsMember(roomID, client.UserID)
	if err != nil || !isMember {
		client.SendEvent(string(domain.EventError), gin.H{"error": "You are not a member of this room"})
		return
	}

	h.hub.JoinRoom(client.ID, roomID)

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

	msgType := domain.MessageTypeText
	if p.Type == string(domain.MessageTypeImage) || p.Type == string(domain.MessageTypeFile) || p.Type == string(domain.MessageTypeAudio) {
		msgType = domain.MessageType(p.Type)
	}

	if msgType == domain.MessageTypeText {
		if p.Content == "" || len(p.Content) > 2000 {
			client.SendEvent(string(domain.EventError), gin.H{"error": "Message content must be 1-2000 characters"})
			return
		}
	} else {
		if p.FileURL == "" {
			client.SendEvent(string(domain.EventError), gin.H{"error": "file_url is required for file/image/audio messages"})
			return
		}
		if len(p.Content) > 2000 {
			client.SendEvent(string(domain.EventError), gin.H{"error": "Caption must be at most 2000 characters"})
			return
		}
	}

	roomID, err := uuid.Parse(p.RoomID)
	if err != nil {
		client.SendEvent(string(domain.EventError), gin.H{"error": "Invalid room ID"})
		return
	}

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
		Type:     msgType,
		FileURL:  p.FileURL,
		FileName: p.FileName,
		FileSize: p.FileSize,
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

	go h.roomRepo.TouchLastMessageAt(roomID)

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

	members, err := h.roomRepo.GetMembers(roomID)
	if err != nil {
		return
	}

	for _, member := range members {
		if excludeClientID != uuid.Nil && h.hub.ClientIDForUser(member.UserID) == excludeClientID {
			continue
		}
		h.hub.SendToUser(member.UserID, data)
	}
}

func (h *Handler) broadcastUserStatus(userID uuid.UUID, username, eventType string) {
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
