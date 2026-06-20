package handler

import (
	"encoding/json"
	"time"
	"net/http"
	"strconv"
	"time"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"
	"chatapp/internal/service"
	ws "chatapp/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authService *service.AuthService
	dmService   *service.DMService
	hub         *ws.Hub
}

func NewAuthHandler(authService *service.AuthService, dmService *service.DMService, hub *ws.Hub) *AuthHandler {
	return &AuthHandler{authService: authService, dmService: dmService, hub: hub}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	resp, err := h.authService.Register(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	resp, err := h.authService.Login(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}


func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("token_jti")
	exp, _ := c.Get("token_exp")

	jtiStr, _ := jti.(string)
	expTime, ok := exp.(time.Time)
	if !ok {
		expTime = time.Now().Add(7 * 24 * time.Hour)
	}

	if err := h.authService.Logout(jtiStr, expTime); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// SearchUsers handles GET /api/v1/users/search?q=username
func (h *AuthHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	userID := middleware.GetUserID(c)

	users, err := h.authService.SearchUsers(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Use live WebSocket connection status instead of the (possibly stale) DB flag
	for i := range users {
		users[i].IsOnline = h.hub.IsUserOnline(users[i].ID)
	}

	c.JSON(http.StatusOK, gin.H{"data": users})
}

// GET /api/v1/users/:id — view another user's public profile
func (h *AuthHandler) GetUserByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.authService.GetProfile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Use live WebSocket connection status instead of the (possibly stale) DB flag
	user.IsOnline = h.hub.IsUserOnline(user.ID)

	c.JSON(http.StatusOK, user)
}

// PATCH /api/v1/auth/username
func (h *AuthHandler) UpdateUsername(c *gin.Context) {
	var req domain.UpdateUsernameRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}
	userID := middleware.GetUserID(c)
	user, err := h.authService.UpdateUsername(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	go h.notifyProfileUpdated(user)
	c.JSON(http.StatusOK, user)
}

// PATCH /api/v1/auth/email
func (h *AuthHandler) UpdateEmail(c *gin.Context) {
	var req domain.UpdateEmailRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}
	userID := middleware.GetUserID(c)
	user, err := h.authService.UpdateEmail(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

// PATCH /api/v1/auth/password
func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	var req domain.UpdatePasswordRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}
	userID := middleware.GetUserID(c)
	if err := h.authService.UpdatePassword(userID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

// PATCH /api/v1/auth/me — update avatar and/or status
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req domain.UpdateProfileRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	userID := middleware.GetUserID(c)
	user, err := h.authService.UpdateProfile(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Notify DM partners in real-time so their sidebar/profile reflects the change
	go h.notifyProfileUpdated(user)

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) notifyProfileUpdated(user *domain.User) {
	partnerIDs, err := h.dmService.GetAllDMPartnerIDs(user.ID)
	if err != nil {
		return
	}

	data, err := json.Marshal(map[string]interface{}{
		"type":    string(domain.EventProfileUpdated),
		"payload": user,
	})
	if err != nil {
		return
	}

	for _, partnerID := range partnerIDs {
		h.hub.SendToUser(partnerID, data)
	}
}

// ---- Room Handler ----

type RoomHandler struct {
	roomService *service.RoomService
}

func NewRoomHandler(roomService *service.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req domain.CreateRoomRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	userID := middleware.GetUserID(c)
	room, err := h.roomService.CreateRoom(req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, room)
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 50 {
		limit = 50
	}

	result, err := h.roomService.GetRooms(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	room, err := h.roomService.GetRoom(roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	c.JSON(http.StatusOK, room)
}

func (h *RoomHandler) GetMyRooms(c *gin.Context) {
	userID := middleware.GetUserID(c)
	rooms, err := h.roomService.GetMyRooms(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rooms})
}

func (h *RoomHandler) JoinRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.roomService.JoinRoom(roomID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined room successfully"})
}

func (h *RoomHandler) LeaveRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.roomService.LeaveRoom(roomID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left room successfully"})
}

func (h *RoomHandler) DeleteRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.roomService.DeleteRoom(roomID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room deleted successfully"})
}

// ---- Message Handler ----

type MessageHandler struct {
	messageService *service.MessageService
	roomRepo       domain.RoomRepository
	hub            *ws.Hub
}

func NewMessageHandler(messageService *service.MessageService, roomRepo domain.RoomRepository, hub *ws.Hub) *MessageHandler {
	return &MessageHandler{messageService: messageService, roomRepo: roomRepo, hub: hub}
}

func (h *MessageHandler) GetMessages(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit > 100 {
		limit = 100
	}

	userID := middleware.GetUserID(c)
	result, err := h.messageService.GetMessages(roomID, userID, page, limit)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Include the partner's read_at so the client can mark old messages as read (blue ticks).
	// We fetch all members and find the one that isn't the current user.
	var partnerReadAt *time.Time
	if members, err := h.roomRepo.GetMembers(roomID); err == nil {
		for _, m := range members {
			if m.UserID != userID {
				partnerReadAt = m.ReadAt
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":             result.Data,
		"total":            result.Total,
		"page":             result.Page,
		"limit":            result.Limit,
		"total_pages":      result.TotalPages,
		"partner_read_at":  partnerReadAt,
	})
}

func (h *MessageHandler) EditMessage(c *gin.Context) {
	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var req domain.EditMessageRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	userID := middleware.GetUserID(c)
	message, err := h.messageService.EditMessage(msgID, userID, req)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	go h.broadcastToRoomMembers(message.RoomID, domain.EventEditMessage, message)

	c.JSON(http.StatusOK, message)
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	userID := middleware.GetUserID(c)

	// Fetch the message first so we know its room for broadcasting after deletion
	roomID, err := h.messageService.GetMessageRoomID(msgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if err := h.messageService.DeleteMessage(msgID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	go h.broadcastToRoomMembers(roomID, domain.EventDeleteMessage, gin.H{"id": msgID, "room_id": roomID})

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})
}

func (h *MessageHandler) broadcastToRoomMembers(roomID uuid.UUID, eventType domain.WSEventType, payload interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    string(eventType),
		"payload": payload,
	})
	if err != nil {
		return
	}

	members, err := h.roomRepo.GetMembers(roomID)
	if err != nil {
		return
	}

	for _, m := range members {
		h.hub.SendToUser(m.UserID, data)
	}
}
