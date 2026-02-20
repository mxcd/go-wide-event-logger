package wideevent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// captureEmitter records emitted events for assertions.
type captureEmitter struct {
	events []*Event
}

func (e *captureEmitter) Emit(evt *Event) {
	e.events = append(e.events, evt)
}

func setupEngine(opts ...Option) (*gin.Engine, *captureEmitter) {
	capture := &captureEmitter{}
	allOpts := append([]Option{WithEmitter(capture)}, opts...)

	engine := gin.New()
	engine.Use(Middleware(allOpts...))
	return engine, capture
}

func TestMiddlewareFullCycle(t *testing.T) {
	engine, capture := setupEngine()

	engine.GET("/api/users", func(c *gin.Context) {
		we := FromGin(c)
		we.Str("endpoint", "list_users").Success()
		c.JSON(200, gin.H{"users": []string{}})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Request-ID", "req-123")
	engine.ServeHTTP(w, req)

	if len(capture.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.events))
	}

	evt := capture.events[0]
	m := fieldsToMap(evt.Fields())

	assertEqual(t, m["request.method"], "GET")
	assertEqual(t, m["request.path"], "/api/users")
	assertEqual(t, m["request.query"], "page=1")
	assertEqual(t, m["request.proto"], "HTTP/1.1")
	assertEqual(t, m["request.user_agent"], "test-agent")
	assertEqual(t, m["request.id"], "req-123")
	assertEqual(t, m["response.status"], 200)
	assertEqual(t, m["endpoint"], "list_users")
	assertEqual(t, m["outcome"], "success")

	if m["response.latency_ms"] == nil {
		t.Error("expected response.latency_ms to be set")
	}
}

func TestMiddlewareRequestFields(t *testing.T) {
	engine, capture := setupEngine()

	engine.POST("/api/data", func(c *gin.Context) {
		c.Status(201)
	})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/data", body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 15
	engine.ServeHTTP(w, req)

	evt := capture.events[0]
	m := fieldsToMap(evt.Fields())

	assertEqual(t, m["request.method"], "POST")
	assertEqual(t, m["request.path"], "/api/data")
	assertEqual(t, m["request.content_length"], int64(15))
}

func TestMiddlewareDualContextStorage(t *testing.T) {
	engine, _ := setupEngine()

	var fromGinEvt, fromCtxEvt *Event

	engine.GET("/test", func(c *gin.Context) {
		fromGinEvt = FromGin(c)
		fromCtxEvt = FromContext(c.Request.Context())
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if fromGinEvt == nil {
		t.Fatal("FromGin returned nil")
	}
	if fromCtxEvt == nil {
		t.Fatal("FromContext returned nil")
	}
	if fromGinEvt != fromCtxEvt {
		t.Error("FromGin and FromContext should return the same event")
	}
}

func TestMiddlewarePreEnricher(t *testing.T) {
	engine, capture := setupEngine(
		WithPreEnricher(func(we *Event, c *gin.Context) {
			we.Str("pre.field", "before_handler")
		}),
	)

	engine.GET("/test", func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	m := fieldsToMap(capture.events[0].Fields())
	assertEqual(t, m["pre.field"], "before_handler")
}

func TestMiddlewarePostEnricher(t *testing.T) {
	engine, capture := setupEngine(
		WithPostEnricher(func(we *Event, c *gin.Context) {
			we.Str("post.field", "after_handler")
		}),
	)

	engine.GET("/test", func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	m := fieldsToMap(capture.events[0].Fields())
	assertEqual(t, m["post.field"], "after_handler")
}

func TestMiddlewareSkipPaths(t *testing.T) {
	engine, capture := setupEngine(WithSkipPaths("/health"))

	engine.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	engine.GET("/api/data", func(c *gin.Context) {
		c.String(200, "data")
	})

	// Health should be skipped
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(w, req)

	if len(capture.events) != 0 {
		t.Errorf("expected 0 events for /health, got %d", len(capture.events))
	}

	// API should emit
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	engine.ServeHTTP(w, req)

	if len(capture.events) != 1 {
		t.Errorf("expected 1 event for /api/data, got %d", len(capture.events))
	}
}

func TestMiddlewareNeverSampleSuppresses(t *testing.T) {
	engine, capture := setupEngine(WithSampler(NeverSample()))

	engine.GET("/test", func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if len(capture.events) != 0 {
		t.Errorf("NeverSample should suppress emission, got %d events", len(capture.events))
	}
}

func TestMiddlewareAlwaysOnErrorKeepsErrors(t *testing.T) {
	engine, capture := setupEngine(WithSampler(AlwaysOnError()))

	engine.GET("/ok", func(c *gin.Context) {
		FromGin(c).Success()
		c.Status(200)
	})
	engine.GET("/fail", func(c *gin.Context) {
		FromGin(c).Set("response.status", 500)
		c.Status(500)
	})

	// Success: should NOT be emitted
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(w, req)
	if len(capture.events) != 0 {
		t.Errorf("AlwaysOnError should not emit success, got %d", len(capture.events))
	}

	// Error: should be emitted
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/fail", nil)
	engine.ServeHTTP(w, req)
	if len(capture.events) != 1 {
		t.Errorf("AlwaysOnError should emit errors, got %d", len(capture.events))
	}
}

func TestMiddlewareIsolation(t *testing.T) {
	engine, capture := setupEngine()

	engine.GET("/a", func(c *gin.Context) {
		FromGin(c).Str("endpoint", "a")
		c.Status(200)
	})
	engine.GET("/b", func(c *gin.Context) {
		FromGin(c).Str("endpoint", "b")
		c.Status(200)
	})

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/a", nil))
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/b", nil))

	if len(capture.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(capture.events))
	}

	m1 := fieldsToMap(capture.events[0].Fields())
	m2 := fieldsToMap(capture.events[1].Fields())

	assertEqual(t, m1["endpoint"], "a")
	assertEqual(t, m2["endpoint"], "b")
}

func TestMiddlewareErrorStatus(t *testing.T) {
	engine, capture := setupEngine()

	engine.GET("/error", func(c *gin.Context) {
		c.JSON(500, gin.H{"error": "internal"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	engine.ServeHTTP(w, req)

	evt := capture.events[0]
	m := fieldsToMap(evt.Fields())
	assertEqual(t, m["response.status"], 500)
}

func TestMiddlewareJSONOutput(t *testing.T) {
	var buf safeBuffer
	engine := gin.New()
	engine.Use(Middleware(WithEmitter(JSONWriterEmitter(&buf))))

	engine.GET("/test", func(c *gin.Context) {
		FromGin(c).Str("custom", "value").Success()
		c.JSON(200, gin.H{})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	var result map[string]any
	if err := json.Unmarshal(buf.buf, &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["level"] != "info" {
		t.Errorf("expected level 'info', got %v", result["level"])
	}

	reqMap, ok := result["request"].(map[string]any)
	if !ok {
		t.Fatal("expected nested 'request' object")
	}
	if reqMap["method"] != "GET" {
		t.Errorf("expected request.method 'GET', got %v", reqMap["method"])
	}

	if result["custom"] != "value" {
		t.Errorf("expected custom 'value', got %v", result["custom"])
	}
}

func TestMiddlewareXRequestID(t *testing.T) {
	engine, capture := setupEngine()

	engine.GET("/test", func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "trace-abc-456")
	engine.ServeHTTP(w, req)

	m := fieldsToMap(capture.events[0].Fields())
	assertEqual(t, m["request.id"], "trace-abc-456")
}

func TestMiddlewareGinErrors(t *testing.T) {
	engine, capture := setupEngine()

	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(http.ErrBodyNotAllowed)
		_ = c.Error(http.ErrHijacked)
		c.Status(500)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	m := fieldsToMap(capture.events[0].Fields())
	assertEqual(t, m["response.gin_errors"], 2)
	if m["response.gin_error.0"] == nil {
		t.Error("expected gin_error.0")
	}
	if m["response.gin_error.1"] == nil {
		t.Error("expected gin_error.1")
	}
}
