package qualitygate

import (
	"encoding/json"
	"testing"
)

func TestNew_NilWhenDisabled(t *testing.T) {
	g := New(Config{Enabled: false})
	if g != nil {
		t.Error("expected nil when disabled")
	}
}

func TestNew_Defaults(t *testing.T) {
	g := New(Config{Enabled: true})
	if g.cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", g.cfg.MaxRetries)
	}
	if g.cfg.OnEmpty != ActionRetry {
		t.Errorf("OnEmpty = %q, want %q", g.cfg.OnEmpty, ActionRetry)
	}
	if g.cfg.OnTruncated != ActionWarn {
		t.Errorf("OnTruncated = %q, want %q", g.cfg.OnTruncated, ActionWarn)
	}
	if g.cfg.OnRefusal != ActionWarn {
		t.Errorf("OnRefusal = %q, want %q", g.cfg.OnRefusal, ActionWarn)
	}
}

func makeResponse(content, finishReason string) []byte {
	resp := map[string]any{
		"choices": []map[string]any{
			{
				"finish_reason": finishReason,
				"message": map[string]string{
					"content": content,
				},
			},
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestCheck_GoodResponse(t *testing.T) {
	g := New(Config{Enabled: true})
	body := makeResponse("The capital of France is Paris.", "stop")
	issue := g.Check(body)
	if issue != nil {
		t.Errorf("expected no issue, got %+v", issue)
	}
}

func TestCheck_EmptyContent(t *testing.T) {
	g := New(Config{Enabled: true})
	body := makeResponse("", "stop")
	issue := g.Check(body)
	if issue == nil {
		t.Fatal("expected issue for empty content")
	}
	if issue.Type != "empty" {
		t.Errorf("Type = %q, want %q", issue.Type, "empty")
	}
	if issue.Action != ActionRetry {
		t.Errorf("Action = %q, want %q", issue.Action, ActionRetry)
	}
}

func TestCheck_NoChoices(t *testing.T) {
	g := New(Config{Enabled: true})
	body, _ := json.Marshal(map[string]any{"choices": []any{}})
	issue := g.Check(body)
	if issue == nil {
		t.Fatal("expected issue for no choices")
	}
	if issue.Type != "empty" {
		t.Errorf("Type = %q, want %q", issue.Type, "empty")
	}
}

func TestCheck_Truncated(t *testing.T) {
	g := New(Config{Enabled: true})
	body := makeResponse("This is a long response that got cut", "length")
	issue := g.Check(body)
	if issue == nil {
		t.Fatal("expected issue for truncated response")
	}
	if issue.Type != "truncated" {
		t.Errorf("Type = %q, want %q", issue.Type, "truncated")
	}
	if issue.Action != ActionWarn {
		t.Errorf("Action = %q, want %q", issue.Action, ActionWarn)
	}
}

func TestCheck_Refusal(t *testing.T) {
	g := New(Config{Enabled: true})
	body := makeResponse("I cannot help with that request.", "stop")
	issue := g.Check(body)
	if issue == nil {
		t.Fatal("expected issue for refusal")
	}
	if issue.Type != "refusal" {
		t.Errorf("Type = %q, want %q", issue.Type, "refusal")
	}
}

func TestCheck_RefusalVariants(t *testing.T) {
	g := New(Config{Enabled: true})
	variants := []string{
		"I can't assist with that.",
		"I'm unable to provide that information.",
		"As an AI, I don't have access to that.",
		"As a language model, I cannot do that.",
	}
	for _, v := range variants {
		body := makeResponse(v, "stop")
		issue := g.Check(body)
		if issue == nil {
			t.Errorf("expected refusal for %q", v)
		}
	}
}

func TestCheck_CustomActions(t *testing.T) {
	g := New(Config{
		Enabled:     true,
		OnEmpty:     ActionReject,
		OnTruncated: ActionRetry,
		OnRefusal:   ActionReject,
	})

	body := makeResponse("", "stop")
	issue := g.Check(body)
	if issue.Action != ActionReject {
		t.Errorf("OnEmpty Action = %q, want %q", issue.Action, ActionReject)
	}

	body = makeResponse("text", "length")
	issue = g.Check(body)
	if issue.Action != ActionRetry {
		t.Errorf("OnTruncated Action = %q, want %q", issue.Action, ActionRetry)
	}
}

func TestCheck_InvalidJSON(t *testing.T) {
	g := New(Config{Enabled: true})
	issue := g.Check([]byte(`not json`))
	if issue != nil {
		t.Error("expected nil for invalid JSON")
	}
}
