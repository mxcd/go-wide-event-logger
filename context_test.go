package wideevent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewContextFromContextRoundTrip(t *testing.T) {
	evt := Begin(nil, "roundtrip")
	ctx := NewContext(context.Background(), evt)

	got := FromContext(ctx)
	if got != evt {
		t.Error("FromContext should return the same event")
	}
}

func TestFromContextEmpty(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Error("FromContext on empty context should return nil")
	}
}

func TestFromGinWithKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	evt := Begin(nil, "gin_key")
	c.Set(ginContextKey, evt)

	got := FromGin(c)
	if got != evt {
		t.Error("FromGin should return event from gin context key")
	}
}

func TestFromGinFallbackToRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	evt := Begin(nil, "fallback")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), evt))
	c.Request = req

	got := FromGin(c)
	if got != evt {
		t.Error("FromGin should fall back to request context")
	}
}

func TestFromGinNilWhenEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	got := FromGin(c)
	if got != nil {
		t.Error("FromGin should return nil when no event")
	}
}
