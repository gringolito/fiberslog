# fiberslog

Fiber middleware for structured request logging with `slog`.

## Install

```bash
go get github.com/gringolito/fiberslog
```

## Usage

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gringolito/fiberslog"
    "golang.org/x/exp/slog"
)

app := fiber.New()
logger := slog.Default()

app.Use(fiberslog.New(fiberslog.Config{
    Logger: logger,
    Fields: []string{"latency", "status", "method", "path", "requestId"},
}))
```

## Features

- Configurable logged fields
- URI skip list
- Conditional body and response body logging
- Automatic level mapping based on HTTP status code

## Security

**`requestHeaders`** logs all request headers verbatim, including `Authorization`, `Cookie`, and API keys. Use `SkipHeaders` to redact sensitive ones:

```go
app.Use(fiberslog.New(fiberslog.Config{
    Fields:      []string{"latency", "status", "method", "requestHeaders"},
    SkipHeaders: []string{"Authorization", "Cookie"},
}))
```

Header name matching is case-insensitive.
