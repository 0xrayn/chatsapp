package handler

import (
	"net/http"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"
	"chatapp/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DMHandler struct {
	dmService *service.DMService
}

func NewDMHandler(dmService *service.DMService) *DMHandler {
	return &DMHandler{dmService: dmService}
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
	room, err := h.dmService.GetOrCreateDM(senderID, recipientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, room)
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
