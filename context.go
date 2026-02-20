package wideevent

import (
	"context"

	"github.com/gin-gonic/gin"
)

type contextKey struct{}

const ginContextKey = "wideevent"

// NewContext returns a new context with the event stored in it.
func NewContext(ctx context.Context, evt *Event) context.Context {
	return context.WithValue(ctx, contextKey{}, evt)
}

// FromContext retrieves the event from a standard context.
// Returns nil if no event is stored.
func FromContext(ctx context.Context) *Event {
	if evt, ok := ctx.Value(contextKey{}).(*Event); ok {
		return evt
	}
	return nil
}

// FromGin retrieves the event from a Gin context.
// It first checks the Gin key store, then falls back to the request context.
// Returns nil if no event is found.
func FromGin(c *gin.Context) *Event {
	if v, ok := c.Get(ginContextKey); ok {
		if evt, ok := v.(*Event); ok {
			return evt
		}
	}
	return FromContext(c.Request.Context())
}
