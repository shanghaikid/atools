package trace

import (
	"sync"
	"testing"
)

func TestNewGeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tr := New()
		if len(tr.ID) != 12 {
			t.Errorf("trace ID length = %d, want 12", len(tr.ID))
		}
		if ids[tr.ID] {
			t.Errorf("duplicate trace ID: %s", tr.ID)
		}
		ids[tr.ID] = true
	}
}

func TestSpanLifecycle(t *testing.T) {
	tr := New()
	tr.StartSpan("test_span").Set("key", "value").End()

	spans := tr.Spans()
	if len(spans) != 1 {
		t.Fatalf("Spans() = %d spans, want 1", len(spans))
	}

	s := spans[0]
	if s.Name != "test_span" {
		t.Errorf("span name = %q, want %q", s.Name, "test_span")
	}
	if s.Metadata["key"] != "value" {
		t.Errorf("span metadata[key] = %v, want %q", s.Metadata["key"], "value")
	}
	if s.DurationMS < 0 {
		t.Errorf("span duration = %d, want >= 0", s.DurationMS)
	}
}

func TestMultipleSpans(t *testing.T) {
	tr := New()
	tr.StartSpan("a").Set("x", 1).End()
	tr.StartSpan("b").Set("y", 2).End()
	tr.StartSpan("c").End()

	spans := tr.Spans()
	if len(spans) != 3 {
		t.Fatalf("Spans() = %d spans, want 3", len(spans))
	}
	if spans[0].Name != "a" || spans[1].Name != "b" || spans[2].Name != "c" {
		t.Errorf("span names = [%s, %s, %s], want [a, b, c]", spans[0].Name, spans[1].Name, spans[2].Name)
	}
}

func TestNilTraceSafety(t *testing.T) {
	var tr *Trace

	// All operations on nil trace should be no-ops
	handle := tr.StartSpan("should_not_panic")
	handle.Set("key", "value")
	handle.End()

	spans := tr.Spans()
	if spans != nil {
		t.Errorf("nil trace Spans() = %v, want nil", spans)
	}
}

func TestConcurrentSpans(t *testing.T) {
	tr := New()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			tr.StartSpan("concurrent").Set("i", i).End()
		}(i)
	}
	wg.Wait()

	spans := tr.Spans()
	if len(spans) != n {
		t.Errorf("Spans() = %d spans, want %d", len(spans), n)
	}
}

func TestSpansReturnsCopy(t *testing.T) {
	tr := New()
	tr.StartSpan("a").End()

	spans1 := tr.Spans()
	tr.StartSpan("b").End()
	spans2 := tr.Spans()

	if len(spans1) != 1 {
		t.Errorf("first Spans() = %d, want 1", len(spans1))
	}
	if len(spans2) != 2 {
		t.Errorf("second Spans() = %d, want 2", len(spans2))
	}
}
