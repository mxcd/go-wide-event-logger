package wideevent

import (
	"math/rand/v2"
	"sync/atomic"
)

// Sampler decides whether a completed event should be emitted.
type Sampler interface {
	ShouldSample(evt *Event) bool
}

// SamplerFunc is an adapter to allow the use of ordinary functions as Samplers.
type SamplerFunc func(evt *Event) bool

func (f SamplerFunc) ShouldSample(evt *Event) bool { return f(evt) }

// AlwaysSample returns a sampler that always emits.
func AlwaysSample() Sampler {
	return SamplerFunc(func(_ *Event) bool { return true })
}

// NeverSample returns a sampler that never emits.
func NeverSample() Sampler {
	return SamplerFunc(func(_ *Event) bool { return false })
}

// AlwaysOnError returns a sampler that emits if the event has an error.
func AlwaysOnError() Sampler {
	return SamplerFunc(func(evt *Event) bool {
		return evt.HasError()
	})
}

// AlwaysOnStatus returns a sampler that emits if the response status matches any of the given codes.
func AlwaysOnStatus(codes ...int) Sampler {
	set := make(map[int]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return SamplerFunc(func(evt *Event) bool {
		_, ok := set[evt.StatusCode()]
		return ok
	})
}

// Rate returns a sampler that emits every Nth event.
func Rate(n int) Sampler {
	if n <= 0 {
		return NeverSample()
	}
	if n == 1 {
		return AlwaysSample()
	}
	var counter atomic.Int64
	return SamplerFunc(func(_ *Event) bool {
		return counter.Add(1)%int64(n) == 0
	})
}

// Probability returns a sampler that emits with the given probability (0.0 to 1.0).
func Probability(p float64) Sampler {
	if p <= 0 {
		return NeverSample()
	}
	if p >= 1 {
		return AlwaysSample()
	}
	return SamplerFunc(func(_ *Event) bool {
		return rand.Float64() < p
	})
}

// CompositeSampler returns a sampler that emits if ANY of the given samplers say yes (OR logic).
func CompositeSampler(samplers ...Sampler) Sampler {
	return SamplerFunc(func(evt *Event) bool {
		for _, s := range samplers {
			if s.ShouldSample(evt) {
				return true
			}
		}
		return false
	})
}
