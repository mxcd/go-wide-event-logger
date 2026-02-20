package wideevent

import "github.com/gin-gonic/gin"

type middlewareConfig struct {
	emitter      Emitter
	sampler      Sampler
	preEnricher  func(*Event, *gin.Context)
	postEnricher func(*Event, *gin.Context)
	skipPaths    map[string]struct{}
}

// Option configures the wide event middleware.
type Option func(*middlewareConfig)

// WithEmitter sets the emitter for the middleware. Default: JSONStdoutEmitter.
func WithEmitter(emitter Emitter) Option {
	return func(cfg *middlewareConfig) {
		cfg.emitter = emitter
	}
}

// WithSampler sets the tail sampler. Default: AlwaysSample.
func WithSampler(sampler Sampler) Option {
	return func(cfg *middlewareConfig) {
		cfg.sampler = sampler
	}
}

// WithPreEnricher sets a function called before the handler to add fields.
func WithPreEnricher(fn func(*Event, *gin.Context)) Option {
	return func(cfg *middlewareConfig) {
		cfg.preEnricher = fn
	}
}

// WithPostEnricher sets a function called after the handler to add fields.
func WithPostEnricher(fn func(*Event, *gin.Context)) Option {
	return func(cfg *middlewareConfig) {
		cfg.postEnricher = fn
	}
}

// WithSkipPaths sets paths that should not generate wide events.
func WithSkipPaths(paths ...string) Option {
	return func(cfg *middlewareConfig) {
		for _, p := range paths {
			cfg.skipPaths[p] = struct{}{}
		}
	}
}
