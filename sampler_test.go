package wideevent

import (
	"errors"
	"testing"
)

func TestAlwaysSample(t *testing.T) {
	s := AlwaysSample()
	evt := Begin(nil, "test")
	if !s.ShouldSample(evt) {
		t.Error("AlwaysSample should return true")
	}
}

func TestNeverSample(t *testing.T) {
	s := NeverSample()
	evt := Begin(nil, "test")
	if s.ShouldSample(evt) {
		t.Error("NeverSample should return false")
	}
}

func TestAlwaysOnErrorWithError(t *testing.T) {
	s := AlwaysOnError()
	evt := Begin(nil, "test")
	evt.Failure(errors.New("fail"))
	if !s.ShouldSample(evt) {
		t.Error("AlwaysOnError should return true for error events")
	}
}

func TestAlwaysOnErrorWithoutError(t *testing.T) {
	s := AlwaysOnError()
	evt := Begin(nil, "test")
	evt.Success()
	if s.ShouldSample(evt) {
		t.Error("AlwaysOnError should return false for success events")
	}
}

func TestAlwaysOnStatusMatching(t *testing.T) {
	s := AlwaysOnStatus(500, 503)
	evt := Begin(nil, "test")
	evt.Set("response.status", 500)
	if !s.ShouldSample(evt) {
		t.Error("AlwaysOnStatus should return true for matching status")
	}
}

func TestAlwaysOnStatusNonMatching(t *testing.T) {
	s := AlwaysOnStatus(500, 503)
	evt := Begin(nil, "test")
	evt.Set("response.status", 200)
	if s.ShouldSample(evt) {
		t.Error("AlwaysOnStatus should return false for non-matching status")
	}
}

func TestRateSampler(t *testing.T) {
	s := Rate(10)
	sampled := 0
	for i := 0; i < 1000; i++ {
		evt := Begin(nil, "test")
		if s.ShouldSample(evt) {
			sampled++
		}
	}
	// Exactly 1/10 of 1000 = 100
	if sampled != 100 {
		t.Errorf("expected 100 sampled, got %d", sampled)
	}
}

func TestRateZero(t *testing.T) {
	s := Rate(0)
	evt := Begin(nil, "test")
	if s.ShouldSample(evt) {
		t.Error("Rate(0) should never sample")
	}
}

func TestRateOne(t *testing.T) {
	s := Rate(1)
	evt := Begin(nil, "test")
	if !s.ShouldSample(evt) {
		t.Error("Rate(1) should always sample")
	}
}

func TestProbabilityZero(t *testing.T) {
	s := Probability(0.0)
	sampled := 0
	for i := 0; i < 100; i++ {
		evt := Begin(nil, "test")
		if s.ShouldSample(evt) {
			sampled++
		}
	}
	if sampled != 0 {
		t.Errorf("Probability(0) should never sample, got %d", sampled)
	}
}

func TestProbabilityOne(t *testing.T) {
	s := Probability(1.0)
	sampled := 0
	for i := 0; i < 100; i++ {
		evt := Begin(nil, "test")
		if s.ShouldSample(evt) {
			sampled++
		}
	}
	if sampled != 100 {
		t.Errorf("Probability(1) should always sample, got %d", sampled)
	}
}

func TestProbabilityHalf(t *testing.T) {
	s := Probability(0.5)
	sampled := 0
	total := 10000
	for i := 0; i < total; i++ {
		evt := Begin(nil, "test")
		if s.ShouldSample(evt) {
			sampled++
		}
	}
	// With 10000 trials, expect ~5000 ± reasonable margin
	ratio := float64(sampled) / float64(total)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("Probability(0.5) expected ~50%%, got %.1f%%", ratio*100)
	}
}

func TestCompositeSamplerOR(t *testing.T) {
	// Error sampler + rate sampler: should sample errors even if rate doesn't match
	s := CompositeSampler(AlwaysOnError(), NeverSample())

	errEvt := Begin(nil, "test")
	errEvt.Failure(errors.New("fail"))
	if !s.ShouldSample(errEvt) {
		t.Error("composite should sample error events via AlwaysOnError")
	}

	okEvt := Begin(nil, "test")
	okEvt.Success()
	if s.ShouldSample(okEvt) {
		t.Error("composite should not sample success events when NeverSample is only other option")
	}
}
