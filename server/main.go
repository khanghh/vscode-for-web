package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {

	rootDir := os.Getenv("ROOT_DIR")
	if rootDir == "" {
		rootDir = "/tmp/remotefs" // default
		os.MkdirAll(rootDir, 0755)
	}

	lfs := NewLocalFileService(rootDir)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "*",
	}))
	// Serve static files from ./static directory
	app.Static("/", "../dist")

	// File system routes
	if err := SetupRoutes(app, lfs); err != nil {
		log.Fatal(err)
	}

	log.Fatal(app.Listen(":3000"))
}
