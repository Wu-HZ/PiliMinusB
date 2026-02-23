package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"piliminusb/config"
	"piliminusb/database"
	"piliminusb/handler"
	"piliminusb/middleware"
	"piliminusb/model"
)

func main() {
	cfg := config.Get()

	// Database
	database.Init()
	database.DB.AutoMigrate(&model.User{})

	// Router
	r := gin.Default()

	// Public routes
	auth := r.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
	}

	// Protected routes (all future Phase 1-4 endpoints go here)
	api := r.Group("/")
	api.Use(middleware.Auth())
	{
		// Phase 1: Watch Later  — will be added here
		// Phase 2: History      — will be added here
		// Phase 3: Favorites    — will be added here
		// Phase 4: Follow       — will be added here
	}

	log.Printf("PiliMinusB server starting on :%s", cfg.Server.Port)
	r.Run(":" + cfg.Server.Port)
}
