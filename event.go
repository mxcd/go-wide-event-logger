package wideevent

import (
	"fmt"
	"sync"
	"time"
)

type field struct {
	key   string
	value any
}

// Event represents a single wide event that accumulates structured fields
// throughout a unit of work and emits them as one rich log line.
type Event struct {
	mu        sync.Mutex
	fields    []field
	emitter   Emitter
	startTime time.Time
	emitted   bool
}

// Begin creates a new standalone event for background tasks or cron jobs.
// The caller is responsible for calling Emit() when the work is done.
func Begin(emitter Emitter, name string) *Event {
	e := &Event{
		fields:    make([]field, 0, 32),
		emitter:   emitter,
		startTime: time.Now(),
	}
	e.fields = append(e.fields, field{key: "name", value: name})
	return e
}

// Emit finalizes the event and sends it to the emitter.
// Automatically sets the duration field. Safe to call multiple times; only the first call emits.
func (e *Event) Emit() {
	e.mu.Lock()
	if e.emitted {
		e.mu.Unlock()
		return
	}
	e.emitted = true
	dur := time.Since(e.startTime)
	e.fields = append(e.fields, field{key: "duration", value: dur})
	e.mu.Unlock()

	if e.emitter != nil {
		e.emitter.Emit(e)
	}
}

// Set adds a field with any value. The key can contain dots for nested JSON output.
func (e *Event) Set(key string, value any) *Event {
	e.mu.Lock()
	e.fields = append(e.fields, field{key: key, value: value})
	e.mu.Unlock()
	return e
}

// Str adds a string field.
func (e *Event) Str(key, value string) *Event {
	return e.Set(key, value)
}

// Int adds an int field.
func (e *Event) Int(key string, value int) *Event {
	return e.Set(key, value)
}

// Int64 adds an int64 field.
func (e *Event) Int64(key string, value int64) *Event {
	return e.Set(key, value)
}

// Float64 adds a float64 field.
func (e *Event) Float64(key string, value float64) *Event {
	return e.Set(key, value)
}

// Bool adds a bool field.
func (e *Event) Bool(key string, value bool) *Event {
	return e.Set(key, value)
}

// Dur adds a time.Duration field.
func (e *Event) Dur(key string, value time.Duration) *Event {
	return e.Set(key, value)
}

// Time adds a time.Time field.
func (e *Event) Time(key string, value time.Time) *Event {
	return e.Set(key, value)
}

// Err adds an error field. If err is nil, this is a no-op.
func (e *Event) Err(key string, err error) *Event {
	if err == nil {
		return e
	}
	return e.Set(key, err.Error())
}

// Interface adds a field with any value (alias for Set).
func (e *Event) Interface(key string, value any) *Event {
	return e.Set(key, value)
}

// UUID adds a field from any fmt.Stringer (typically a UUID type).
func (e *Event) UUID(key string, value fmt.Stringer) *Event {
	return e.Set(key, value.String())
}

// Success sets the outcome field to "success".
func (e *Event) Success() *Event {
	return e.Set("outcome", "success")
}

// Failure sets the outcome field to "failure" and records the error.
func (e *Event) Failure(err error) *Event {
	e.Set("outcome", "failure")
	if err != nil {
		e.Set("error", err.Error())
	}
	return e
}

// UserID sets the user.id field from any fmt.Stringer.
func (e *Event) UserID(id fmt.Stringer) *Event {
	return e.Set("user.id", id.String())
}

// UserName sets the user.name field.
func (e *Event) UserName(name string) *Event {
	return e.Set("user.name", name)
}

// UserRole sets the user.role field.
func (e *Event) UserRole(role string) *Event {
	return e.Set("user.role", role)
}

// HasError returns true if the event has an outcome of "failure" or a response status >= 500.
func (e *Event) HasError() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range e.fields {
		if f.key == "outcome" && f.value == "failure" {
			return true
		}
		if f.key == "response.status" {
			if code, ok := f.value.(int); ok && code >= 500 {
				return true
			}
		}
	}
	return false
}

// StatusCode returns the response status code, or 0 if not set.
func (e *Event) StatusCode() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range e.fields {
		if f.key == "response.status" {
			if code, ok := f.value.(int); ok {
				return code
			}
		}
	}
	return 0
}

// Fields returns a snapshot copy of all fields.
func (e *Event) Fields() []field {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]field, len(e.fields))
	copy(cp, e.fields)
	return cp
}
