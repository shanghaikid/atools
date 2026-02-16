package compressor

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_NilWhenDisabled(t *testing.T) {
	c := New(Config{Enabled: false}, nil)
	if c != nil {
		t.Error("expected nil when disabled")
	}
}

func TestNew_Defaults(t *testing.T) {
	c := New(Config{Enabled: true}, nil)
	if c.cfg.ThresholdTokens != 50000 {
		t.Errorf("ThresholdTokens = %d, want 50000", c.cfg.ThresholdTokens)
	}
	if c.cfg.KeepRecent != 10 {
		t.Errorf("KeepRecent = %d, want 10", c.cfg.KeepRecent)
	}
	if c.cfg.SummaryModel != "gpt-4o-mini" {
		t.Errorf("SummaryModel = %q, want %q", c.cfg.SummaryModel, "gpt-4o-mini")
	}
}

func TestCompress_UnderThreshold(t *testing.T) {
	c := New(Config{Enabled: true, ThresholdTokens: 1000}, nil)
	msgs, _ := json.Marshal([]Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	})

	result := c.Compress(msgs)
	if string(result) != string(msgs) {
		t.Error("should not compress under threshold")
	}
}

func TestCompress_OverThreshold(t *testing.T) {
	// Use a mock summarizer
	summarizer := func(model string, msgs []Message) (string, error) {
		return "Summary of earlier conversation.", nil
	}

	c := New(Config{
		Enabled:         true,
		ThresholdTokens: 10, // very low threshold
		KeepRecent:      2,
	}, summarizer)

	msgs, _ := json.Marshal([]Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "This is message one with some content."},
		{Role: "assistant", Content: "This is reply one with some content."},
		{Role: "user", Content: "This is message two with some content."},
		{Role: "assistant", Content: "This is reply two with some content."},
		{Role: "user", Content: "Recent message."},
		{Role: "assistant", Content: "Recent reply."},
	})

	result := c.Compress(msgs)

	var compressed []Message
	if err := json.Unmarshal(result, &compressed); err != nil {
		t.Fatalf("unmarshal compressed: %v", err)
	}

	// Should have: system + summary + 2 recent = 4 messages
	if len(compressed) != 4 {
		t.Fatalf("len = %d, want 4", len(compressed))
	}

	// First should be original system
	if compressed[0].Content != "You are a helpful assistant." {
		t.Errorf("first msg = %q, want system", compressed[0].Content)
	}

	// Second should be summary
	if !strings.Contains(compressed[1].Content, "Conversation summary") {
		t.Errorf("second msg should be summary, got %q", compressed[1].Content)
	}

	// Last two should be recent
	if compressed[2].Content != "Recent message." {
		t.Errorf("third msg = %q, want 'Recent message.'", compressed[2].Content)
	}
}

func TestCompress_NotEnoughToCompress(t *testing.T) {
	c := New(Config{
		Enabled:         true,
		ThresholdTokens: 1, // very low
		KeepRecent:      10,
	}, nil)

	msgs, _ := json.Marshal([]Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	})

	result := c.Compress(msgs)
	// Only 2 conversation msgs, keep_recent=10, so can't compress
	if string(result) != string(msgs) {
		t.Error("should not compress when fewer msgs than keep_recent")
	}
}

func TestCompress_FallbackSummarize(t *testing.T) {
	// No summarize function — uses fallback
	c := New(Config{
		Enabled:         true,
		ThresholdTokens: 5,
		KeepRecent:      1,
	}, nil)

	msgs, _ := json.Marshal([]Message{
		{Role: "user", Content: "First message with content."},
		{Role: "assistant", Content: "First reply with content."},
		{Role: "user", Content: "Second message with content."},
		{Role: "assistant", Content: "Second reply."},
		{Role: "user", Content: "Recent."},
	})

	result := c.Compress(msgs)
	var compressed []Message
	if err := json.Unmarshal(result, &compressed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Should have summary + 1 recent = 2 messages
	if len(compressed) != 2 {
		t.Fatalf("len = %d, want 2", len(compressed))
	}

	if !strings.Contains(compressed[0].Content, "[user]") {
		t.Error("fallback summary should contain role prefixes")
	}
}

func TestEstimateTokens(t *testing.T) {
	// "Hello world" = 2 words * 1.3 = 2.6 → 2
	tokens := estimateTokens("Hello world")
	if tokens != 2 {
		t.Errorf("tokens = %d, want 2", tokens)
	}

	tokens = estimateTokens("")
	if tokens != 0 {
		t.Errorf("empty = %d, want 0", tokens)
	}
}

func TestCompress_InvalidJSON(t *testing.T) {
	c := New(Config{Enabled: true}, nil)
	input := json.RawMessage(`not json`)
	result := c.Compress(input)
	if string(result) != string(input) {
		t.Error("should return original on invalid JSON")
	}
}
