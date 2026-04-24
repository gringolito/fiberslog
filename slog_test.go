package fiberslog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestNew_LogFields(t *testing.T) {
	defaultHandler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}

	tests := []struct {
		name        string
		field       string
		routeMethod string
		routePath   string
		buildReq    func() *http.Request
		handler     fiber.Handler
		assert      func(*testing.T, map[string]any)
	}{
		{
			name:  "latency",
			field: "latency",
			assert: func(t *testing.T, attrs map[string]any) {
				v, ok := attrs["latency"]
				require.True(t, ok)
				_, ok = v.(time.Duration)
				require.True(t, ok, "latency must be time.Duration")
			},
		},
		{
			name:  "pid",
			field: "pid",
			assert: func(t *testing.T, attrs map[string]any) {
				v, ok := attrs["pid"]
				require.True(t, ok)
				n, ok := v.(int64)
				require.True(t, ok, "pid must be int64")
				require.Positive(t, n)
			},
		},
		{
			name:  "ip",
			field: "ip",
			assert: func(t *testing.T, attrs map[string]any) {
				_, ok := attrs["ip"]
				require.True(t, ok)
				require.IsType(t, "", attrs["ip"])
			},
		},
		{
			name:  "ips",
			field: "ips",
			buildReq: func() *http.Request {
				r := httptest.NewRequest(fiber.MethodGet, "/", nil)
				r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
				return r
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "1.2.3.4, 5.6.7.8", attrs["ips"])
			},
		},
		{
			name:  "host",
			field: "host",
			assert: func(t *testing.T, attrs map[string]any) {
				_, ok := attrs["host"]
				require.True(t, ok)
				require.IsType(t, "", attrs["host"])
			},
		},
		{
			name:      "path",
			field:     "path",
			routePath: "/items",
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "/items", attrs["path"])
			},
		},
		{
			name:      "url",
			field:     "url",
			routePath: "/items",
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodGet, "/items?q=42", nil)
			},
			assert: func(t *testing.T, attrs map[string]any) {
				s, ok := attrs["url"].(string)
				require.True(t, ok)
				require.Contains(t, s, "q=42")
			},
		},
		{
			name:  "user-agent",
			field: "user-agent",
			buildReq: func() *http.Request {
				r := httptest.NewRequest(fiber.MethodGet, "/", nil)
				r.Header.Set("User-Agent", "test-agent/1.0")
				return r
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "test-agent/1.0", attrs["user-agent"])
			},
		},
		{
			name:  "referer",
			field: "referer",
			buildReq: func() *http.Request {
				r := httptest.NewRequest(fiber.MethodGet, "/", nil)
				r.Header.Set("Referer", "https://example.com")
				return r
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "https://example.com", attrs["referer"])
			},
		},
		{
			name:  "protocol",
			field: "protocol",
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "http", attrs["protocol"])
			},
		},
		{
			name:  "port",
			field: "port",
			assert: func(t *testing.T, attrs map[string]any) {
				_, ok := attrs["port"]
				require.True(t, ok)
				require.IsType(t, "", attrs["port"])
			},
		},
		{
			name:  "status",
			field: "status",
			assert: func(t *testing.T, attrs map[string]any) {
				require.EqualValues(t, 200, attrs["status"])
			},
		},
		{
			name:        "method",
			field:       "method",
			routeMethod: fiber.MethodPost,
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodPost, "/", nil)
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "POST", attrs["method"])
			},
		},
		{
			name:      "route",
			field:     "route",
			routePath: "/items/:id",
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodGet, "/items/42", nil)
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "/items/:id", attrs["route"])
			},
		},
		{
			name:  "requestId",
			field: "requestId",
			handler: func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderXRequestID, "req-abc-123")
				return c.SendStatus(fiber.StatusOK)
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, "req-abc-123", attrs["requestId"])
			},
		},
		{
			name:  "requestHeaders",
			field: "requestHeaders",
			buildReq: func() *http.Request {
				r := httptest.NewRequest(fiber.MethodGet, "/", nil)
				r.Header.Set("X-Custom-Header", "custom-value")
				return r
			},
			assert: func(t *testing.T, attrs map[string]any) {
				v, ok := attrs["X-Custom-Header"]
				require.True(t, ok, "X-Custom-Header must be present")
				require.Equal(t, []byte("custom-value"), v)
			},
		},
		{
			name:      "queryParams",
			field:     "queryParams",
			routePath: "/search",
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodGet, "/search?foo=bar&baz=qux", nil)
			},
			assert: func(t *testing.T, attrs map[string]any) {
				s, ok := attrs["queryParams"].(string)
				require.True(t, ok)
				require.Contains(t, s, "foo=bar")
			},
		},
		{
			name:        "body",
			field:       "body",
			routeMethod: fiber.MethodPost,
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("hello body"))
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, []byte("hello body"), attrs["body"])
			},
		},
		{
			name:  "responseBody",
			field: "responseBody",
			handler: func(c *fiber.Ctx) error {
				return c.SendString("hello response")
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.Equal(t, []byte("hello response"), attrs["responseBody"])
			},
		},
		{
			name:        "bytesReceived",
			field:       "bytesReceived",
			routeMethod: fiber.MethodPost,
			buildReq: func() *http.Request {
				return httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("hello"))
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.EqualValues(t, 5, attrs["bytesReceived"])
			},
		},
		{
			name:  "bytesSent",
			field: "bytesSent",
			handler: func(c *fiber.Ctx) error {
				return c.SendString("hello")
			},
			assert: func(t *testing.T, attrs map[string]any) {
				require.EqualValues(t, 5, attrs["bytesSent"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, records := newCaptureLogger()

			app := fiber.New()
			app.Use(New(Config{Logger: logger, Fields: []string{tt.field}}))

			method := tt.routeMethod
			if method == "" {
				method = fiber.MethodGet
			}
			path := tt.routePath
			if path == "" {
				path = "/"
			}
			handler := tt.handler
			if handler == nil {
				handler = defaultHandler
			}
			app.Add(method, path, handler)

			var req *http.Request
			if tt.buildReq != nil {
				req = tt.buildReq()
			} else {
				req = httptest.NewRequest(method, path, nil)
			}

			_, err := app.Test(req)
			require.NoError(t, err)
			require.Len(t, *records, 1)

			tt.assert(t, (*records)[0].Attrs)
		})
	}
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

func TestNew_ConcurrentRequests(t *testing.T) {
	logger, records := newCaptureLogger()

	app := fiber.New()
	app.Use(New(Config{
		Logger: logger,
		Fields: []string{"status", "method", "pid", "latency", "url"},
	}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			resp, reqErr := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			if reqErr != nil {
				t.Errorf("unexpected error: %v", reqErr)
				return
			}
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()

	require.Len(t, *records, n)
	for _, rec := range *records {
		require.EqualValues(t, 200, rec.Attrs["status"])
		require.Equal(t, "GET", rec.Attrs["method"])
		require.Positive(t, rec.Attrs["pid"])
	}
}

func TestNew_PropagatesNextError(t *testing.T) {
	tests := []struct {
		name            string
		handlerErr      error
		expectedStatus  int
		expectedLevel   slog.Level
		expectedMsg     string
		expectedErrAttr string
	}{
		{
			name:            "bad_request",
			handlerErr:      fiber.ErrBadRequest,
			expectedStatus:  fiber.StatusBadRequest,
			expectedLevel:   slog.LevelWarn,
			expectedMsg:     "Client error",
			expectedErrAttr: "Bad Request",
		},
		{
			name:            "not_found",
			handlerErr:      fiber.ErrNotFound,
			expectedStatus:  fiber.StatusNotFound,
			expectedLevel:   slog.LevelWarn,
			expectedMsg:     "Client error",
			expectedErrAttr: "Not Found",
		},
		{
			name:            "internal_server_error",
			handlerErr:      fiber.ErrInternalServerError,
			expectedStatus:  fiber.StatusInternalServerError,
			expectedLevel:   slog.LevelError,
			expectedMsg:     "Server error",
			expectedErrAttr: "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, records := newCaptureLogger()

			app := fiber.New()
			app.Use(New(Config{Logger: logger, Fields: []string{"status"}}))
			app.Get("/", func(c *fiber.Ctx) error {
				return tt.handlerErr
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, resp.StatusCode)

			require.Len(t, *records, 1)
			rec := (*records)[0]
			require.Equal(t, tt.expectedLevel, rec.Level)
			require.Equal(t, tt.expectedMsg, rec.Msg)

			errAttr, ok := rec.Attrs["error"]
			require.True(t, ok, "log record must contain 'error' attribute")
			require.Equal(t, tt.expectedErrAttr, errAttr)
		})
	}
}
