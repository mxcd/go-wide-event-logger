package wideevent

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that creates a wide event per request,
// enriches it with request/response metadata, and emits it after the handler completes.
func Middleware(opts ...Option) gin.HandlerFunc {
	cfg := &middlewareConfig{
		emitter:   JSONStdoutEmitter(),
		sampler:   AlwaysSample(),
		skipPaths: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, skip := cfg.skipPaths[path]; skip {
			c.Next()
			return
		}

		start := time.Now()

		evt := &Event{
			fields:    make([]Field, 0, 32),
			emitter:   cfg.emitter,
			startTime: start,
		}

		// Store in both Gin context and request context
		c.Set(ginContextKey, evt)
		c.Request = c.Request.WithContext(NewContext(c.Request.Context(), evt))

		// Set request fields
		evt.Set("request.method", c.Request.Method)
		evt.Set("request.path", path)
		evt.Set("request.query", c.Request.URL.RawQuery)
		evt.Set("request.host", c.Request.Host)
		evt.Set("request.proto", c.Request.Proto)
		evt.Set("request.content_length", c.Request.ContentLength)
		evt.Set("request.client_ip", c.ClientIP())
		evt.Set("request.user_agent", c.Request.UserAgent())
		evt.Set("request.id", c.GetHeader("X-Request-ID"))
		evt.Set("request.start_time", start)

		if cfg.preEnricher != nil {
			cfg.preEnricher(evt, c)
		}

		// Finalize in a defer so a panicking handler still produces an event. The panic is
		// re-raised afterwards so an outer Recovery middleware keeps handling it.
		defer func() {
			rec := recover()
			status := c.Writer.Status()
			if rec != nil {
				evt.Failure(fmt.Errorf("panic: %v", rec))
				if !c.Writer.Written() {
					status = http.StatusInternalServerError
				}
			}

			// Set response fields
			latency := time.Since(start)
			evt.Set("response.status", status)
			evt.Set("response.body_size", c.Writer.Size())
			evt.Set("response.latency", latency)
			evt.Set("response.latency_ms", float64(latency.Nanoseconds())/1e6)

			ginErrors := c.Errors
			evt.Set("response.gin_errors", len(ginErrors))
			for i, e := range ginErrors {
				evt.Set(fmt.Sprintf("response.gin_error.%d", i), e.Error())
			}

			if cfg.postEnricher != nil {
				cfg.postEnricher(evt, c)
			}

			// Emit is a no-op if the handler already emitted the event.
			if cfg.sampler.ShouldSample(evt) {
				evt.Emit()
			}

			if rec != nil {
				panic(rec)
			}
		}()

		c.Next()
	}
}
