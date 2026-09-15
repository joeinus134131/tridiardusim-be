package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/user/ardusim-backend/internal/api"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName:   "ArduSim API v1.0",
		BodyLimit: 2 * 1024 * 1024,
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://127.0.0.1:3000,http://localhost:3001,http://127.0.0.1:3001",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	dir := os.Getenv("PROJECT_DIR")
	if dir == "" {
		dir = "data/projects"
	}
	api.RegisterProjects(app, dir)

	// Start Server
	go func() {
		log.Println("Starting backend server on port 8080...")
		if err := app.Listen("127.0.0.1:8080"); err != nil {
			log.Fatalf("Fiber failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
}
