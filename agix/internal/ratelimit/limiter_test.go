package ratelimit

import (
	"testing"
	"time"
)

func TestNew_NilOnEmpty(t *testing.T) {
	if l := New(nil); l != nil {
		t.Error("expected nil limiter for nil limits")
	}
	if l := New(map[string]Limit{}); l != nil {
		t.Error("expected nil limiter for empty limits")
	}
}

func TestAllow_NoAgent(t *testing.T) {
	l := New(map[string]Limit{"agent1": {RequestsPerMinute: 1}})
	r := l.Allow("")
	if !r.Allowed {
		t.Error("expected allow for empty agent")
	}
}

func TestAllow_NoLimitConfigured(t *testing.T) {
	l := New(map[string]Limit{"agent1": {RequestsPerMinute: 1}})
	r := l.Allow("agent2")
	if !r.Allowed {
		t.Error("expected allow for unconfigured agent")
	}
}

func TestAllow_PerMinute(t *testing.T) {
	l := New(map[string]Limit{"agent1": {RequestsPerMinute: 3}})

	for i := 0; i < 3; i++ {
		r := l.Allow("agent1")
		if !r.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	r := l.Allow("agent1")
	if r.Allowed {
		t.Error("4th request should be denied")
	}
	if r.RetryAfter < time.Second {
		t.Error("RetryAfter should be at least 1 second")
	}
	if r.Err == nil {
		t.Error("expected error on denied request")
	}
}

func TestAllow_PerHour(t *testing.T) {
	l := New(map[string]Limit{"agent1": {RequestsPerHour: 2}})

	for i := 0; i < 2; i++ {
		r := l.Allow("agent1")
		if !r.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	r := l.Allow("agent1")
	if r.Allowed {
		t.Error("3rd request should be denied")
	}
}

func TestAllow_IndependentAgents(t *testing.T) {
	l := New(map[string]Limit{
		"agent1": {RequestsPerMinute: 1},
		"agent2": {RequestsPerMinute: 1},
	})

	r1 := l.Allow("agent1")
	if !r1.Allowed {
		t.Error("agent1 first request should be allowed")
	}

	r2 := l.Allow("agent2")
	if !r2.Allowed {
		t.Error("agent2 first request should be allowed (independent)")
	}

	r1b := l.Allow("agent1")
	if r1b.Allowed {
		t.Error("agent1 second request should be denied")
	}
}

func TestWindow_Evict(t *testing.T) {
	w := &window{
		timestamps: []time.Time{
			time.Now().Add(-2 * time.Hour),
			time.Now().Add(-30 * time.Minute),
			time.Now().Add(-5 * time.Minute),
		},
	}
	w.evict(time.Now(), time.Hour)
	if len(w.timestamps) != 2 {
		t.Errorf("expected 2 timestamps after evict, got %d", len(w.timestamps))
	}
}
