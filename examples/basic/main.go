package main

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gringolito/fiberslog"
)

func main() {
	app := fiber.New()

	// requestHeaders with SkipHeaders — redacts Authorization and Cookie.
	app.Use(fiberslog.New(fiberslog.Config{
		Logger:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		Fields:      []string{"method", "url", "status", "latency", "requestHeaders"},
		SkipHeaders: []string{"Authorization", "Cookie"},
	}))

	app.Get("/hello", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello, World!"})
	})

	if err := app.Listen(":8000"); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
