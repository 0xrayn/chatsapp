package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chatapp/internal/database"
	"chatapp/internal/handler"
	applogger "chatapp/internal/logger"
	"chatapp/internal/middleware"
	"chatapp/internal/repository"
	"chatapp/internal/router"
	"chatapp/internal/service"
	ws "chatapp/internal/websocket"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Init structured logger
	applogger.Init(env)
	applogger.Log.Info().Str("env", env).Msg("Starting ChatApp")

	// Connect DB
	db, err := database.NewPostgresDB()
	if err != nil {
		applogger.Log.Fatal().Err(err).Msg("Failed to connect database")
	}

	// Migrate
	if err := database.AutoMigrate(db); err != nil {
		applogger.Log.Fatal().Err(err).Msg("Failed to migrate database")
	}

	// Repos
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	msgRepo := repository.NewMessageRepository(db)

	// WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Services
	authService := service.NewAuthService(userRepo)
	roomService := service.NewRoomService(roomRepo)
	msgService := service.NewMessageService(msgRepo, roomRepo)
	dmService := service.NewDMService(roomRepo, userRepo, msgRepo, hub)

	// Handlers
	authHandler := handler.NewAuthHandler(authService, dmService, hub)
	roomHandler := handler.NewRoomHandler(roomService)
	msgHandler := handler.NewMessageHandler(msgService)
	dmHandler := handler.NewDMHandler(dmService, hub)
	uploadHandler := handler.NewUploadHandler()
	wsHandler := ws.NewHandler(hub, msgRepo, roomRepo, userRepo)

	// Rate limiters
	apiLimiter := middleware.NewAPIRateLimiter()
	authLimiter := middleware.NewAuthRateLimiter()

	// Setup router
	r := router.SetupRoutes(router.Config{
		AuthHandler:   authHandler,
		RoomHandler:   roomHandler,
		MsgHandler:    msgHandler,
		DMHandler:     dmHandler,
		UploadHandler: uploadHandler,
		WSHandler:     wsHandler,
		APILimiter:    apiLimiter,
		AuthLimiter:   authLimiter,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		applogger.Log.Info().
			Str("addr", srv.Addr).
			Msg("🚀 Server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applogger.Log.Fatal().Err(err).Msg("Server error")
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	applogger.Log.Info().Str("signal", sig.String()).Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		applogger.Log.Error().Err(err).Msg("Server forced shutdown")
	}

	// Close DB
	sqlDB, _ := db.DB()
	sqlDB.Close()

	applogger.Log.Info().Msg("✅ Server exited cleanly")
}
