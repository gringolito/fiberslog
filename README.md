# fiberslog

[![CI](https://github.com/gringolito/fiberslog/actions/workflows/ci.yaml/badge.svg)](https://github.com/gringolito/fiberslog/actions/workflows/ci.yaml)
[![CodeQL](https://github.com/gringolito/fiberslog/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/gringolito/fiberslog/actions/workflows/github-code-scanning/codeql)
[![codecov](https://codecov.io/gh/gringolito/fiberslog/branch/master/graph/badge.svg)](https://codecov.io/gh/gringolito/fiberslog)
[![Go Reference](https://pkg.go.dev/badge/github.com/gringolito/fiberslog.svg)](https://pkg.go.dev/github.com/gringolito/fiberslog)
[![Latest Release](https://img.shields.io/github/v/release/gringolito/fiberslog)](https://github.com/gringolito/fiberslog/releases/latest)
![Go 1.24+](https://img.shields.io/badge/go-%3E%3D1.24-blue)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Fiber middleware for structured request logging with `slog`.

## Install

```bash
go get github.com/gringolito/fiberslog
```

## Usage

```go
import (
    "log/slog"

    "github.com/gofiber/fiber/v2"
    "github.com/gringolito/fiberslog"
)

app := fiber.New()
logger := slog.Default()

app.Use(fiberslog.New(fiberslog.Config{
    Logger: logger,
    Fields: []string{"latency", "status", "method", "path", "requestId"},
}))
```

## Features

- Configurable logged fields (22 built-in fields, see reference below)
- URI skip list
- Conditional body and response body logging
- Automatic level mapping based on HTTP status code (2xx→Info, 4xx→Warn, 5xx→Error)
- Sensitive header redaction via `SkipHeaders`

## Field Reference

The `Fields` config option accepts any combination of the names below. Unknown names are silently ignored. The default set is `latency`, `status`, `method`, `url`, `pid`.

| Field | Log attribute key | Type | Description |
| --- | --- | --- | --- |
| `latency` | `latency` | `time.Duration` | Time to process the request |
| `status` | `status` | `int` | HTTP response status code |
| `method` | `method` | `string` | HTTP request method |
| `url` | `url` | `string` | Full original URL (includes query string) |
| `path` | `path` | `string` | Request path without query string |
| `pid` | `pid` | `int` | Process ID |
| `ip` | `ip` | `string` | Client IP address |
| `ips` | `ips` | `string` | `X-Forwarded-For` header value |
| `host` | `host` | `string` | Request hostname |
| `port` | `port` | `string` | Request port |
| `protocol` | `protocol` | `string` | Request protocol (`http` or `https`) |
| `referer` | `referer` | `string` | `Referer` header value |
| `user-agent` | `user-agent` | `string` | `User-Agent` header value |
| `requestId` | `requestId` | `string` | `X-Request-ID` response header |
| `route` | `route` | `string` | Matched Fiber route pattern |
| `queryParams` | `queryParams` | `string` | Serialized query parameters |
| `body` | `body` | `[]byte` | Request body bytes |
| `responseBody` | `responseBody` | `[]byte` | Response body bytes |
| `bytesReceived` | `bytesReceived` | `int` | Request body size in bytes |
| `bytesSent` | `bytesSent` | `int` | Response body size in bytes |
| `requestHeaders` | _(one attr per header)_ | `[]byte` | All request headers (**see Security**) |
| `responseHeaders` | _(one attr per header)_ | `[]byte` | All response headers (**see Security**) |

## Security

**`requestHeaders` and `responseHeaders`** log all headers verbatim, including `Authorization`, `Cookie`, and API keys. Use `SkipHeaders` to redact sensitive ones:

```go
app.Use(fiberslog.New(fiberslog.Config{
    Fields:      []string{"latency", "status", "method", "requestHeaders"},
    SkipHeaders: []string{"Authorization", "Cookie"},
}))
```

Header name matching is case-insensitive.

## Contributing

Contributions are welcome! Feel free to:

- [Open an issue](https://github.com/gringolito/fiberslog/issues) to report a bug or request a new feature
- Submit a pull request — please include tests for any new behavior
- Suggest ideas or improvements by starting a [discussion](https://github.com/gringolito/fiberslog/discussions)

---

_This project has moved from the Beerware License to MIT, but the spirit lives on: if we ever meet and you think this stuff is worth it, you're still very welcome to buy me a beer._
