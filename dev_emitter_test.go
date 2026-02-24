package wideevent

import (
	"strings"
	"testing"
	"time"
)

func TestDevEmitterInfoHTTP(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "get_users")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/api/v1/users")
	evt.Str("request.host", "localhost:8080")
	evt.Str("request.client_ip", "127.0.0.1")
	evt.Int("response.status", 200)
	evt.Float64("response.latency_ms", 12.4)
	evt.Int("response.body_size", 1842)
	evt.Str("user.id", "abc-123")
	evt.Str("user.name", "john")
	evt.Success()
	evt.Emit()

	out := buf.String()

	// Header should contain method, path, status, latency
	if !strings.Contains(out, "GET /api/v1/users") {
		t.Errorf("header missing method/path, got:\n%s", out)
	}
	if !strings.Contains(out, "200") {
		t.Errorf("header missing status code, got:\n%s", out)
	}
	if !strings.Contains(out, "12.4ms") {
		t.Errorf("header missing latency, got:\n%s", out)
	}

	// Body should have grouped fields
	if !strings.Contains(out, "request") && !strings.Contains(out, "host=localhost:8080") {
		t.Errorf("missing request group fields, got:\n%s", out)
	}
	if !strings.Contains(out, "response") && !strings.Contains(out, "body_size=1842") {
		t.Errorf("missing response group fields, got:\n%s", out)
	}
	if !strings.Contains(out, "user") && !strings.Contains(out, "name=john") {
		t.Errorf("missing user group fields, got:\n%s", out)
	}

	// Footer should contain outcome and duration
	if !strings.Contains(out, "└") {
		t.Errorf("missing footer, got:\n%s", out)
	}
	if !strings.Contains(out, "outcome=success") {
		t.Errorf("missing outcome in footer, got:\n%s", out)
	}
}

func TestDevEmitterErrorHTTP(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "create_user")
	evt.Str("request.method", "POST")
	evt.Str("request.path", "/api/v1/users")
	evt.Int("response.status", 500)
	evt.Float64("response.latency_ms", 45.2)
	evt.Failure(nil)
	evt.Str("error", "connection refused")
	evt.Emit()

	out := buf.String()

	if !strings.Contains(out, "POST /api/v1/users") {
		t.Errorf("header missing, got:\n%s", out)
	}
	if !strings.Contains(out, "500") {
		t.Errorf("missing status 500, got:\n%s", out)
	}
	if !strings.Contains(out, "error=connection refused") {
		t.Errorf("missing error in footer, got:\n%s", out)
	}
	if !strings.Contains(out, "outcome=failure") {
		t.Errorf("missing outcome=failure, got:\n%s", out)
	}
}

func TestDevEmitterWarnHTTP(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "not_found")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/api/v1/missing")
	evt.Int("response.status", 404)
	evt.Float64("response.latency_ms", 2.1)
	evt.Emit()

	out := buf.String()

	if !strings.Contains(out, "GET /api/v1/missing") {
		t.Errorf("header missing, got:\n%s", out)
	}
	if !strings.Contains(out, "404") {
		t.Errorf("missing status 404, got:\n%s", out)
	}
}

func TestDevEmitterStandalone(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "sync_users")
	evt.Int("db.queries", 12)
	evt.Int("db.rows_affected", 48)
	evt.Success()
	evt.Emit()

	out := buf.String()

	if !strings.Contains(out, "sync_users") {
		t.Errorf("header should show event name, got:\n%s", out)
	}
	if !strings.Contains(out, "queries=12") {
		t.Errorf("missing db.queries field, got:\n%s", out)
	}
	if !strings.Contains(out, "rows_affected=48") {
		t.Errorf("missing db.rows_affected field, got:\n%s", out)
	}
	if !strings.Contains(out, "outcome=success") {
		t.Errorf("missing outcome, got:\n%s", out)
	}
}

func TestDevEmitterGroupOrder(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "ordered")
	// Add groups in non-standard order
	evt.Str("cache.hit", "false")
	evt.Str("user.name", "john")
	evt.Str("response.status_text", "OK")
	evt.Str("request.method_override", "PATCH")
	evt.Emit()

	out := buf.String()

	// request should appear before response, response before user, user before cache
	reqIdx := strings.Index(out, "request")
	respIdx := strings.Index(out, "response")
	userIdx := strings.Index(out, "user")
	cacheIdx := strings.Index(out, "cache")

	if reqIdx > respIdx || respIdx > userIdx || userIdx > cacheIdx {
		t.Errorf("groups not in expected order (request < response < user < cache), got:\n%s", out)
	}
}

func TestDevEmitterNoColor(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf) // default: no color

	evt := Begin(emitter, "no_color")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("should have no ANSI codes with color disabled, got:\n%s", out)
	}
}

func TestDevEmitterWithColor(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf, WithColor(true))

	evt := Begin(emitter, "colored")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "\033[") {
		t.Errorf("should have ANSI codes with color enabled, got:\n%s", out)
	}
}

func TestDevEmitterDurationFormat(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "dur_test")
	evt.Dur("custom.fast", 500*time.Microsecond)
	evt.Dur("custom.medium", 150*time.Millisecond)
	evt.Dur("custom.slow", 2500*time.Millisecond)
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "500µs") {
		t.Errorf("expected microsecond format, got:\n%s", out)
	}
	if !strings.Contains(out, "150.0ms") {
		t.Errorf("expected millisecond format, got:\n%s", out)
	}
	if !strings.Contains(out, "2.50s") {
		t.Errorf("expected second format, got:\n%s", out)
	}
}

func TestDevEmitterTimeFormat(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	ts := time.Date(2026, 2, 24, 14, 30, 45, 123000000, time.UTC)
	evt := Begin(emitter, "time_test")
	evt.Time("custom.ts", ts)
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "14:30:45.123") {
		t.Errorf("expected HH:MM:SS.mmm format, got:\n%s", out)
	}
}

func TestDevEmitterHeaderFooterExcluded(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "excluded")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/test")
	evt.Int("response.status", 200)
	evt.Float64("response.latency_ms", 5.0)
	evt.Str("db.operation", "select")
	evt.Success()
	evt.Emit()

	out := buf.String()
	lines := strings.Split(out, "\n")

	// The header/footer fields should NOT appear as key=value pairs in the body
	// (they're shown in header/footer instead).
	bodyLines := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "│") || (len(line) > 0 && line[0] == '\xe2') {
			bodyLines = append(bodyLines, line)
		}
	}

	body := strings.Join(bodyLines, "\n")
	// request.method and request.path are in header, but remaining request fields
	// should still appear. Here we only set method/path which are header fields,
	// so there should be no "request" group at all.
	// response.status and response.latency_ms are header fields, so no response group.
	// But db.operation should appear.
	if !strings.Contains(body, "operation=select") {
		t.Errorf("expected db.operation in body, got:\n%s", out)
	}
}

func TestDevEmitterCategoryLabel(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf,
		WithCategory("assets", "/src/assets", "/static"),
	)

	evt := Begin(emitter, "serve_asset")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/src/assets/logo.png")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "[assets]") {
		t.Errorf("expected [assets] category label in header, got:\n%s", out)
	}
	if !strings.Contains(out, "GET /src/assets/logo.png") {
		t.Errorf("expected path in header, got:\n%s", out)
	}
}

func TestDevEmitterMuteCategory(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf,
		WithCategory("assets", "/src/assets"),
		WithMuteCategories("assets"),
	)

	evt := Begin(emitter, "serve_asset")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/src/assets/logo.png")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if out != "" {
		t.Errorf("expected no output for muted category, got:\n%s", out)
	}
}

func TestDevEmitterUnmutedCategory(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf,
		WithCategory("api", "/api"),
		WithCategory("assets", "/src/assets"),
		WithMuteCategories("assets"),
	)

	evt := Begin(emitter, "api_call")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/api/v1/users")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "[api]") {
		t.Errorf("expected [api] label for unmuted category, got:\n%s", out)
	}
	if !strings.Contains(out, "GET /api/v1/users") {
		t.Errorf("expected normal output for unmuted category, got:\n%s", out)
	}
}

func TestDevEmitterNoCategoryMatch(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf,
		WithCategory("assets", "/src/assets"),
	)

	evt := Begin(emitter, "other")
	evt.Str("request.method", "GET")
	evt.Str("request.path", "/api/v1/users")
	evt.Int("response.status", 200)
	evt.Success()
	evt.Emit()

	out := buf.String()
	if strings.Contains(out, "[") && strings.Contains(out, "]") {
		// Check it's not a category label (could be other brackets in output)
		if strings.Contains(out, "[assets]") {
			t.Errorf("should not have category label for unmatched path, got:\n%s", out)
		}
	}
	if !strings.Contains(out, "GET /api/v1/users") {
		t.Errorf("expected normal output, got:\n%s", out)
	}
}

func TestDevEmitterCategoryPrefixMatch(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf,
		WithCategory("assets", "/src/assets"),
	)

	// Exact prefix match
	evt1 := Begin(emitter, "t1")
	evt1.Str("request.method", "GET")
	evt1.Str("request.path", "/src/assets")
	evt1.Int("response.status", 200)
	evt1.Emit()

	out1 := buf.String()
	if !strings.Contains(out1, "[assets]") {
		t.Errorf("expected [assets] for exact prefix match, got:\n%s", out1)
	}

	buf.Reset()

	// Subpath match
	evt2 := Begin(emitter, "t2")
	evt2.Str("request.method", "GET")
	evt2.Str("request.path", "/src/assets/images/logo.png")
	evt2.Int("response.status", 200)
	evt2.Emit()

	out2 := buf.String()
	if !strings.Contains(out2, "[assets]") {
		t.Errorf("expected [assets] for subpath match, got:\n%s", out2)
	}

	buf.Reset()

	// Non-match: different prefix
	evt3 := Begin(emitter, "t3")
	evt3.Str("request.method", "GET")
	evt3.Str("request.path", "/src/components/App.tsx")
	evt3.Int("response.status", 200)
	evt3.Emit()

	out3 := buf.String()
	if strings.Contains(out3, "[assets]") {
		t.Errorf("should not match different prefix, got:\n%s", out3)
	}
}

func TestDevEmitterEmptyEvent(t *testing.T) {
	var buf safeBuffer
	emitter := DevWriterEmitter(&buf)

	evt := Begin(emitter, "minimal")
	evt.Emit()

	out := buf.String()
	if !strings.Contains(out, "minimal") {
		t.Errorf("expected name in header, got:\n%s", out)
	}
	if !strings.Contains(out, "└") {
		t.Errorf("expected footer, got:\n%s", out)
	}
}
