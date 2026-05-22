package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "ArduSim API v1.0",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // For MVP. In production, restrict this.
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// API Group
	api := app.Group("/api")

	// Project Routes (Mocked for MVP)
	projects := api.Group("/projects")
	projects.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"projects": []interface{}{}})
	})
	projects.Post("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "created", "id": "mock-uuid"})
	})

	// AI Routes (Mocked for MVP)
	ai := api.Group("/ai")
	ai.Post("/chat", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"reply": "This is a mock AI response from the Go backend."})
	})

	// Start Server
	go func() {
		log.Println("Starting backend server on port 8080...")
		if err := app.Listen(":8080"); err != nil {
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
