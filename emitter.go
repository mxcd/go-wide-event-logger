package wideevent

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Emitter defines the interface for outputting wide events.
type Emitter interface {
	Emit(evt *Event)
}

// EmitterFunc is an adapter to allow the use of ordinary functions as Emitters.
type EmitterFunc func(evt *Event)

func (f EmitterFunc) Emit(evt *Event) { f(evt) }

// JSONStdoutEmitter returns an Emitter that writes one JSON line per event to stdout.
func JSONStdoutEmitter() Emitter {
	return JSONWriterEmitter(os.Stdout)
}

// JSONWriterEmitter returns an Emitter that writes one JSON line per event to the given writer.
func JSONWriterEmitter(w io.Writer) Emitter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return EmitterFunc(func(evt *Event) {
		fields := evt.Fields()
		nested := nestFields(convertFieldValues(fields))
		nested["level"] = inferLevel(evt)
		nested["message"] = "wide_event"
		_ = enc.Encode(nested)
	})
}

// inferLevel determines the log level based on event content.
func inferLevel(evt *Event) string {
	if evt.HasError() {
		return "error"
	}
	code := evt.StatusCode()
	if code >= 500 {
		return "error"
	}
	if code >= 400 {
		return "warn"
	}
	return "info"
}

// convertFieldValues converts typed values to JSON-friendly representations.
func convertFieldValues(fields []field) []field {
	out := make([]field, len(fields))
	for i, f := range fields {
		out[i] = field{key: f.key, value: convertValue(f.value)}
	}
	return out
}

func convertValue(v any) any {
	switch val := v.(type) {
	case time.Duration:
		return val.Seconds() * 1000 // milliseconds as float64
	case time.Time:
		return val.Format(time.RFC3339Nano)
	default:
		return v
	}
}
