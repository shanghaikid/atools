package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// Limit defines rate limits for an agent.
type Limit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	RequestsPerHour   int `yaml:"requests_per_hour"`
}

// Limiter enforces per-agent rate limits using a sliding window counter.
type Limiter struct {
	limits  map[string]Limit
	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	timestamps []time.Time
}

// New creates a Limiter from the given per-agent limits.
// Returns nil if limits is nil or empty.
func New(limits map[string]Limit) *Limiter {
	if len(limits) == 0 {
		return nil
	}
	return &Limiter{
		limits:  limits,
		windows: make(map[string]*window),
	}
}

// Result is returned by Allow when a request is denied.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
	Err        error
}

// Allow checks whether the agent is within its rate limits.
func (l *Limiter) Allow(agent string) Result {
	if agent == "" {
		return Result{Allowed: true}
	}

	limit, ok := l.limits[agent]
	if !ok {
		return Result{Allowed: true}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w := l.getWindow(agent)
	w.evict(now, time.Hour) // keep only last hour of data

	// Check per-minute limit
	if limit.RequestsPerMinute > 0 {
		count := w.countSince(now, time.Minute)
		if count >= limit.RequestsPerMinute {
			retryAfter := w.oldestSince(now, time.Minute).Add(time.Minute).Sub(now)
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			return Result{
				Allowed:    false,
				RetryAfter: retryAfter,
				Err:        fmt.Errorf("rate limit exceeded: %d requests per minute (limit %d)", count, limit.RequestsPerMinute),
			}
		}
	}

	// Check per-hour limit
	if limit.RequestsPerHour > 0 {
		count := w.countSince(now, time.Hour)
		if count >= limit.RequestsPerHour {
			retryAfter := w.oldestSince(now, time.Hour).Add(time.Hour).Sub(now)
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			return Result{
				Allowed:    false,
				RetryAfter: retryAfter,
				Err:        fmt.Errorf("rate limit exceeded: %d requests per hour (limit %d)", count, limit.RequestsPerHour),
			}
		}
	}

	// Record this request
	w.timestamps = append(w.timestamps, now)

	return Result{Allowed: true}
}

func (l *Limiter) getWindow(agent string) *window {
	w, ok := l.windows[agent]
	if !ok {
		w = &window{}
		l.windows[agent] = w
	}
	return w
}

// evict removes timestamps older than the given duration.
func (w *window) evict(now time.Time, maxAge time.Duration) {
	cutoff := now.Add(-maxAge)
	i := 0
	for i < len(w.timestamps) && w.timestamps[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		w.timestamps = w.timestamps[i:]
	}
}

// countSince counts timestamps within the given duration from now.
func (w *window) countSince(now time.Time, d time.Duration) int {
	cutoff := now.Add(-d)
	count := 0
	for _, t := range w.timestamps {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count
}

// oldestSince returns the oldest timestamp within the given duration.
func (w *window) oldestSince(now time.Time, d time.Duration) time.Time {
	cutoff := now.Add(-d)
	for _, t := range w.timestamps {
		if !t.Before(cutoff) {
			return t
		}
	}
	return now
}
