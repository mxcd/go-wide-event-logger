package wideevent

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type testStringer struct{ val string }

func (s testStringer) String() string { return s.val }

func TestConcurrentFieldAccumulation(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)
	evt := Begin(emitter, "concurrent_test")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			evt.Set(fmt.Sprintf("field_%d", n), n)
		}(i)
	}
	wg.Wait()

	fields := evt.Fields()
	// 1 for name + 100 from goroutines
	if len(fields) != 101 {
		t.Errorf("expected 101 fields, got %d", len(fields))
	}

	// Verify all 100 goroutine fields are present
	seen := make(map[string]bool)
	for _, f := range fields {
		seen[f.key] = true
	}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("field_%d", i)
		if !seen[key] {
			t.Errorf("missing field %s", key)
		}
	}
}

func TestMethodChainingReturnsSameEvent(t *testing.T) {
	evt := Begin(nil, "chain_test")

	got := evt.Str("a", "b").Int("c", 1).Bool("d", true).Float64("e", 1.5)
	if got != evt {
		t.Error("chaining should return the same *Event")
	}
}

func TestTypedSetters(t *testing.T) {
	evt := Begin(nil, "typed_test")

	now := time.Now()
	dur := 5 * time.Second

	evt.Str("s", "hello")
	evt.Int("i", 42)
	evt.Int64("i64", int64(99))
	evt.Float64("f64", 3.14)
	evt.Bool("b", true)
	evt.Dur("dur", dur)
	evt.Time("t", now)
	evt.Err("err", errors.New("boom"))
	evt.Interface("iface", map[string]int{"x": 1})
	evt.UUID("uuid", testStringer{"abc-123"})

	fields := evt.Fields()
	m := fieldsToMap(fields)

	assertEqual(t, m["s"], "hello")
	assertEqual(t, m["i"], 42)
	assertEqual(t, m["i64"], int64(99))
	assertEqual(t, m["f64"], 3.14)
	assertEqual(t, m["b"], true)
	assertEqual(t, m["dur"], dur)
	assertEqual(t, m["t"], now)
	assertEqual(t, m["err"], "boom")
	assertEqual(t, m["uuid"], "abc-123")
}

func TestErrNilIsNoOp(t *testing.T) {
	evt := Begin(nil, "err_nil")
	evt.Err("err", nil)

	fields := evt.Fields()
	m := fieldsToMap(fields)
	if _, ok := m["err"]; ok {
		t.Error("Err(nil) should not add a field")
	}
}

func TestConvenienceBuilders(t *testing.T) {
	evt := Begin(nil, "convenience_test")
	evt.Success()
	m := fieldsToMap(evt.Fields())
	assertEqual(t, m["outcome"], "success")

	evt2 := Begin(nil, "failure_test")
	evt2.Failure(errors.New("bad"))
	m2 := fieldsToMap(evt2.Fields())
	assertEqual(t, m2["outcome"], "failure")
	assertEqual(t, m2["error"], "bad")

	evt3 := Begin(nil, "user_test")
	evt3.UserID(testStringer{"uid-1"}).UserName("john").UserRole("admin")
	m3 := fieldsToMap(evt3.Fields())
	assertEqual(t, m3["user.id"], "uid-1")
	assertEqual(t, m3["user.name"], "john")
	assertEqual(t, m3["user.role"], "admin")
}

func TestSetWithDotKeys(t *testing.T) {
	evt := Begin(nil, "dot_test")
	evt.Set("db.operation", "update").Set("db.entity", "user")

	m := fieldsToMap(evt.Fields())
	assertEqual(t, m["db.operation"], "update")
	assertEqual(t, m["db.entity"], "user")
}

func TestHasError(t *testing.T) {
	evt := Begin(nil, "err_test")
	if evt.HasError() {
		t.Error("new event should not have error")
	}

	evt.Failure(errors.New("fail"))
	if !evt.HasError() {
		t.Error("event with failure should have error")
	}

	evt2 := Begin(nil, "status_test")
	evt2.Set("response.status", 500)
	if !evt2.HasError() {
		t.Error("event with status 500 should have error")
	}

	evt3 := Begin(nil, "ok_test")
	evt3.Set("response.status", 200)
	if evt3.HasError() {
		t.Error("event with status 200 should not have error")
	}
}

func TestStatusCode(t *testing.T) {
	evt := Begin(nil, "code_test")
	if evt.StatusCode() != 0 {
		t.Error("new event should have status 0")
	}

	evt.Set("response.status", 404)
	if evt.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", evt.StatusCode())
	}
}

func TestBeginEmitLifecycle(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "lifecycle_test")
	evt.Str("key", "value")
	time.Sleep(5 * time.Millisecond)
	evt.Emit()

	output := buf.String()
	if output == "" {
		t.Error("expected output after Emit()")
	}

	// Duration should be present
	if !containsString(output, "duration") {
		t.Error("expected duration field in output")
	}
}

func TestEmitOnlyOnce(t *testing.T) {
	var buf safeBuffer
	emitter := JSONWriterEmitter(&buf)

	evt := Begin(emitter, "once_test")
	evt.Emit()
	first := buf.String()

	evt.Emit()
	second := buf.String()

	if first != second {
		t.Error("second Emit() should be a no-op")
	}
}

// helpers

func fieldsToMap(fields []field) map[string]any {
	m := make(map[string]any)
	for _, f := range fields {
		m[f.key] = f.value
	}
	return m
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && // avoid trivial matches
		fmt.Sprintf("%s", haystack) != "" &&
		stringContains(haystack, needle)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
