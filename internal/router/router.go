package router

import (
	"net/http"

	"chatapp/internal/handler"
	"chatapp/internal/middleware"
	ws "chatapp/internal/websocket"

	"github.com/gin-gonic/gin"
)

type Config struct {
	AuthHandler    *handler.AuthHandler
	RoomHandler    *handler.RoomHandler
	MsgHandler     *handler.MessageHandler
	DMHandler      *handler.DMHandler
	UploadHandler  *handler.UploadHandler
	WSHandler      *ws.Handler
	APILimiter     *middleware.RateLimiter
	AuthLimiter    *middleware.RateLimiter
}

func SetupRoutes(cfg Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "chatapp"})
	})

	// Static files (WS test client)
	r.Static("/static", "./static")

	v1 := r.Group("/api/v1")

	// Auth (strict rate limit)
	auth := v1.Group("/auth")
	auth.Use(cfg.AuthLimiter.Middleware())
	{
		auth.POST("/register", cfg.AuthHandler.Register)
		auth.POST("/login", cfg.AuthHandler.Login)
	}

	// Protected (general rate limit)
	protected := v1.Group("/")
	protected.Use(middleware.AuthMiddleware())
	protected.Use(cfg.APILimiter.Middleware())
	{
		protected.GET("/auth/me", cfg.AuthHandler.GetProfile)
		protected.PATCH("/auth/me", cfg.AuthHandler.UpdateProfile)
		protected.GET("/users/search", cfg.AuthHandler.SearchUsers)
		protected.POST("/upload", cfg.UploadHandler.Upload)

		// Rooms
		rooms := protected.Group("/rooms")
		{
			rooms.GET("", cfg.RoomHandler.GetRooms)
			rooms.POST("", cfg.RoomHandler.CreateRoom)
			rooms.GET("/me", cfg.RoomHandler.GetMyRooms)
			rooms.GET("/:id", cfg.RoomHandler.GetRoom)
			rooms.DELETE("/:id", cfg.RoomHandler.DeleteRoom)
			rooms.POST("/:id/join", cfg.RoomHandler.JoinRoom)
			rooms.POST("/:id/leave", cfg.RoomHandler.LeaveRoom)
			rooms.GET("/:id/messages", cfg.MsgHandler.GetMessages)
			rooms.POST("/:id/read", cfg.DMHandler.MarkAsRead)
		}

		// Direct Messages
		dm := protected.Group("/dm")
		{
			dm.GET("", cfg.DMHandler.GetMyDMs)
			dm.POST("", cfg.DMHandler.GetOrCreateDM)
		}

		// Messages
		messages := protected.Group("/messages")
		{
			messages.PATCH("/:id", cfg.MsgHandler.EditMessage)
			messages.DELETE("/:id", cfg.MsgHandler.DeleteMessage)
		}
	}

	// WebSocket — own auth (supports ?token= query param, no rate limit on upgrade)
	v1.GET("/ws", middleware.WSAuthMiddleware(), cfg.WSHandler.HandleWS)

	return r
}
