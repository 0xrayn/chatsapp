package handler

import (
	"encoding/json"
	"net/http"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"
	"chatapp/internal/service"
	ws "chatapp/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DMHandler struct {
	dmService *service.DMService
	hub       *ws.Hub
}

func NewDMHandler(dmService *service.DMService, hub *ws.Hub) *DMHandler {
	return &DMHandler{dmService: dmService, hub: hub}
}

// POST /api/v1/dm — get or create a DM room with another user
func (h *DMHandler) GetOrCreateDM(c *gin.Context) {
	var req domain.CreateDMRequest
	if !middleware.BindJSONOrError(c, &req) {
		return
	}

	recipientID, err := uuid.Parse(req.RecipientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipient ID"})
		return
	}

	senderID := middleware.GetUserID(c)
	wasExisting, _ := h.dmService.HasDirectRoom(senderID, recipientID)

	room, err := h.dmService.GetOrCreateDM(senderID, recipientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Notify the recipient in real-time if this is a brand-new conversation
	if !wasExisting {
		h.hub.JoinRoomForUser(senderID, room.ID)
		h.hub.JoinRoomForUser(recipientID, room.ID)
		h.notifyDMCreated(recipientID, senderID, room)
	}

	c.JSON(http.StatusOK, room)
}

func (h *DMHandler) notifyDMCreated(recipientID, senderID uuid.UUID, room *domain.Room) {
	var senderUser domain.User
	for _, m := range room.Members {
		if m.UserID == senderID {
			senderUser = m.User
			break
		}
	}

	payload := map[string]interface{}{
		"room_id": room.ID,
		"sender":  senderUser,
	}

	data, err := json.Marshal(map[string]interface{}{
		"type":    string(domain.EventDMCreated),
		"payload": payload,
	})
	if err != nil {
		return
	}

	h.hub.SendToUser(recipientID, data)
}

// GET /api/v1/dm — list all my DM conversations
func (h *DMHandler) GetMyDMs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	dms, err := h.dmService.GetDMRooms(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dms})
}

// POST /api/v1/rooms/:id/read — mark all messages in a room as read
func (h *DMHandler) MarkAsRead(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.dmService.MarkAsRead(roomID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Notify other room members in real-time that messages were read,
	// using direct user delivery so it works regardless of WS room-join state
	data, err := json.Marshal(map[string]interface{}{
		"type": string(domain.EventMessagesRead),
		"payload": map[string]interface{}{
			"room_id": roomID,
			"user_id": userID,
		},
	})
	if err == nil {
		members, _ := h.dmService.GetRoomMemberIDs(roomID)
		for _, memberID := range members {
			if memberID != userID {
				h.hub.SendToUser(memberID, data)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}
