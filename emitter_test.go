package wideevent

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// safeBuffer is a thread-safe bytes.Buffer for tests
type safeBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (b *safeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = nil
}

func TestJSONOutputStructure(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "test_event")
	evt.Str("endpoint", "get_users").Success()
	evt.Set("response.status", 200)
	evt.Emit()

	var result map[string]any
	if err := json.Unmarshal(buf.buf, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["level"] != "info" {
		t.Errorf("expected level 'info', got %v", result["level"])
	}
	if result["message"] != "wide_event" {
		t.Errorf("expected message 'wide_event', got %v", result["message"])
	}
	if result["name"] != "test_event" {
		t.Errorf("expected name 'test_event', got %v", result["name"])
	}
	if result["outcome"] != "success" {
		t.Errorf("expected outcome 'success', got %v", result["outcome"])
	}
}

func TestJSONNestedOutput(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "nested")
	evt.Set("db.operation", "update").Set("db.entity", "user")
	evt.Emit()

	var result map[string]any
	if err := json.Unmarshal(buf.buf, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	db, ok := result["db"].(map[string]any)
	if !ok {
		t.Fatal("expected nested 'db' object")
	}
	if db["operation"] != "update" {
		t.Errorf("expected operation 'update', got %v", db["operation"])
	}
	if db["entity"] != "user" {
		t.Errorf("expected entity 'user', got %v", db["entity"])
	}
}

func TestJSONDeepNesting(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "deep")
	evt.Set("a.b.c.d", "value")
	evt.Emit()

	var result map[string]any
	if err := json.Unmarshal(buf.buf, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	a := result["a"].(map[string]any)
	b := a["b"].(map[string]any)
	c := b["c"].(map[string]any)
	if c["d"] != "value" {
		t.Errorf("expected 'value', got %v", c["d"])
	}
}

func TestLevelInferenceError(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "error")
	evt.Set("response.status", 500)
	evt.Emit()

	var result map[string]any
	json.Unmarshal(buf.buf, &result)
	if result["level"] != "error" {
		t.Errorf("expected 'error', got %v", result["level"])
	}
}

func TestLevelInferenceWarn(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "warn")
	evt.Set("response.status", 404)
	evt.Emit()

	var result map[string]any
	json.Unmarshal(buf.buf, &result)
	if result["level"] != "warn" {
		t.Errorf("expected 'warn', got %v", result["level"])
	}
}

func TestLevelInferenceFailure(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "failure")
	evt.Failure(nil)
	// outcome = "failure" means HasError() = true
	evt.Set("outcome", "failure")
	evt.Emit()

	var result map[string]any
	json.Unmarshal(buf.buf, &result)
	if result["level"] != "error" {
		t.Errorf("expected 'error', got %v", result["level"])
	}
}

func TestDurationEmittedAsMs(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "dur_test")
	evt.Dur("custom_dur", 150*time.Millisecond)
	evt.Emit()

	var result map[string]any
	json.Unmarshal(buf.buf, &result)

	durVal, ok := result["custom_dur"].(float64)
	if !ok {
		t.Fatal("duration should be a float64")
	}
	// 150ms = 150.0 (approximately)
	if durVal < 140 || durVal > 160 {
		t.Errorf("expected ~150ms, got %f", durVal)
	}
}

func TestTimeEmittedAsRFC3339Nano(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	now := time.Date(2026, 2, 20, 10, 30, 0, 123456789, time.UTC)
	evt := Begin(emitter, "time_test")
	evt.Time("ts", now)
	evt.Emit()

	var result map[string]any
	json.Unmarshal(buf.buf, &result)

	expected := now.Format(time.RFC3339Nano)
	if result["ts"] != expected {
		t.Errorf("expected %s, got %v", expected, result["ts"])
	}
}
