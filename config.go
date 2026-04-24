package fiberslog

import (
	"github.com/gofiber/fiber/v2"
	"log/slog"
)

type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// SkipBody defines a function to skip log  "body" field when returned true.
	//
	// Optional. Default: nil
	SkipBody func(c *fiber.Ctx) bool

	// SkipResBody defines a function to skip log  "resBody" field when returned true.
	//
	// Optional. Default: nil
	SkipResBody func(c *fiber.Ctx) bool

	// GetResBody defines a function to get ResBody.
	//  eg: when use compress middleware, resBody is unreadable. you can set GetResBody func to get readable resBody.
	//
	// Optional. Default: nil
	GetResBody func(c *fiber.Ctx) []byte

	// Skip logging for these uri
	//
	// Optional. Default: nil
	SkipURIs []string

	// Add custom slog logger.
	//
	// Optional. Default: slog.Default()
	Logger *slog.Logger

	// Add fields what you want see.
	//
	// Optional. Default: {"latency", "status", "method", "url", "pid"}
	Fields []string

	// SkipHeaders is a list of header names to exclude from requestHeaders logging.
	// Matching is case-insensitive. Use this to redact sensitive headers such as
	// Authorization or Cookie.
	// Warning: if omitted, ALL headers including credentials will be logged.
	//
	// Optional. Default: nil
	SkipHeaders []string
}

var defaultConfig = Config{
	Logger: slog.Default(),
	Fields: []string{"latency", "status", "method", "url", "pid"},
}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		cfg := defaultConfig
		cfg.Fields = append([]string(nil), defaultConfig.Fields...)
		return cfg
	}

	cfg := config[0]

	if cfg.Logger == nil {
		cfg.Logger = defaultConfig.Logger
	}

	if cfg.Fields == nil {
		cfg.Fields = append([]string(nil), defaultConfig.Fields...)
	}

	return cfg
}
