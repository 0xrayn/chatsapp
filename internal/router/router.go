package router

import (
	"net/http"
	"os"
	"strings"

	"chatapp/internal/handler"
	"chatapp/internal/middleware"
	ws "chatapp/internal/websocket"

	"github.com/gin-gonic/gin"
)

type Config struct {
	AuthHandler   *handler.AuthHandler
	RoomHandler   *handler.RoomHandler
	MsgHandler    *handler.MessageHandler
	DMHandler     *handler.DMHandler
	UploadHandler *handler.UploadHandler
	WSHandler     *ws.Handler
	APILimiter    *middleware.RateLimiter
	AuthLimiter   *middleware.RateLimiter
}

// allowedOrigins reads ALLOWED_ORIGINS from the environment (comma-separated).
// Falls back to localhost only if not set — never allow-all in any mode.
//
// Example .env:
//
//	ALLOWED_ORIGINS=https://app.example.com,https://www.example.com
func allowedOrigins() map[string]bool {
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

func SetupRoutes(cfg Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	origins := allowedOrigins()

	// Global middleware
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())

	// CORS — whitelist only, never allow-all
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Max-Age", "86400")
		}
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

	// Static files
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
		protected.POST("/auth/logout", cfg.AuthHandler.Logout)
		protected.GET("/auth/me", cfg.AuthHandler.GetProfile)
		protected.PATCH("/auth/me", cfg.AuthHandler.UpdateProfile)
		protected.PATCH("/auth/username", cfg.AuthHandler.UpdateUsername)
		protected.PATCH("/auth/email", cfg.AuthHandler.UpdateEmail)
		protected.PATCH("/auth/password", cfg.AuthHandler.UpdatePassword)
		protected.GET("/users/search", cfg.AuthHandler.SearchUsers)
		protected.GET("/users/:id", cfg.AuthHandler.GetUserByID)
		protected.POST("/upload", cfg.UploadHandler.Upload)

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

		dm := protected.Group("/dm")
		{
			dm.GET("", cfg.DMHandler.GetMyDMs)
			dm.POST("", cfg.DMHandler.GetOrCreateDM)
		}

		messages := protected.Group("/messages")
		{
			messages.PATCH("/:id", cfg.MsgHandler.EditMessage)
			messages.DELETE("/:id", cfg.MsgHandler.DeleteMessage)
		}
	}

	// WebSocket — first-message auth (token never in URL)
	v1.GET("/ws", middleware.WSAuthMiddleware(), cfg.WSHandler.HandleWS)

	return r
}
