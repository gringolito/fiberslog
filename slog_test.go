package fiberslog

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"log/slog"
)

type recordedLog struct {
	Level slog.Level
	Msg   string
	Attrs map[string]any
}

type captureHandler struct {
	mu      *sync.Mutex
	records *[]recordedLog
}

func newCaptureLogger() (*slog.Logger, *[]recordedLog) {
	records := make([]recordedLog, 0)
	h := &captureHandler{mu: &sync.Mutex{}, records: &records}
	return slog.New(h), &records
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]any{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	h.mu.Lock()
	*h.records = append(*h.records, recordedLog{
		Level: r.Level,
		Msg:   r.Message,
		Attrs: attrs,
	})
	h.mu.Unlock()

	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return &captureHandler{mu: h.mu, records: h.records}
}

func (h *captureHandler) WithGroup(_ string) slog.Handler {
	return &captureHandler{mu: h.mu, records: h.records}
}

func TestConfigDefault(t *testing.T) {
	cfg := configDefault()
	require.NotNil(t, cfg.Logger)
	require.Equal(t, []string{"latency", "status", "method", "url", "pid"}, cfg.Fields)
}

func TestNew_LogsExpectedSeverityByStatusCode(t *testing.T) {
	logger, records := newCaptureLogger()

	app := fiber.New()
	app.Use(New(Config{Logger: logger, Fields: []string{"status", "method"}}))
	app.Get("/ok", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/bad", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusBadRequest) })
	app.Get("/err", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusInternalServerError) })

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/ok", nil))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/bad", nil))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/err", nil))
	require.NoError(t, err)

	require.Len(t, *records, 3)
	require.Equal(t, slog.LevelInfo, (*records)[0].Level)
	require.Equal(t, "Success", (*records)[0].Msg)
	require.EqualValues(t, 200, (*records)[0].Attrs["status"])

	require.Equal(t, slog.LevelWarn, (*records)[1].Level)
	require.Equal(t, "Client error", (*records)[1].Msg)
	require.EqualValues(t, 400, (*records)[1].Attrs["status"])

	require.Equal(t, slog.LevelError, (*records)[2].Level)
	require.Equal(t, "Server error", (*records)[2].Msg)
	require.EqualValues(t, 500, (*records)[2].Attrs["status"])
}

func TestNew_SkipsWhenNextOrSkipURI(t *testing.T) {
	logger, records := newCaptureLogger()

	app := fiber.New()
	app.Use(New(Config{
		Logger:   logger,
		Fields:   []string{"status"},
		SkipURIs: []string{"/skip"},
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/next"
		},
	}))
	app.Get("/next", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/skip", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/log", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/next", nil))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/skip", nil))
	require.NoError(t, err)
	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/log", nil))
	require.NoError(t, err)

	require.Len(t, *records, 1)
	require.EqualValues(t, 200, (*records)[0].Attrs["status"])
}

func TestNew_SkipHeaders(t *testing.T) {
	logger, records := newCaptureLogger()

	app := fiber.New()
	app.Use(New(Config{
		Logger:      logger,
		Fields:      []string{"requestHeaders"},
		SkipHeaders: []string{"Authorization", "Cookie"},
	}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=abc")
	req.Header.Set("X-Custom-Header", "present")

	_, err := app.Test(req)
	require.NoError(t, err)
	require.Len(t, *records, 1)

	attrs := (*records)[0].Attrs
	_, hasAuth := attrs["Authorization"]
	_, hasCookie := attrs["Cookie"]
	_, hasCustom := attrs["X-Custom-Header"]
	require.False(t, hasAuth, "Authorization header must be redacted")
	require.False(t, hasCookie, "Cookie header must be redacted")
	require.True(t, hasCustom, "X-Custom-Header must be present")
}

func TestNew_SkipBodyAndSkipResBody(t *testing.T) {
	logger, records := newCaptureLogger()

	app := fiber.New()
	app.Use(New(Config{
		Logger:      logger,
		Fields:      []string{"body", "responseBody"},
		SkipBody:    func(*fiber.Ctx) bool { return true },
		SkipResBody: func(*fiber.Ctx) bool { return true },
	}))
	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendString("response")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/", nil))
	require.NoError(t, err)

	require.Len(t, *records, 1)
	_, hasBody := (*records)[0].Attrs["body"]
	_, hasResBody := (*records)[0].Attrs["responseBody"]
	require.False(t, hasBody)
	require.False(t, hasResBody)
}
