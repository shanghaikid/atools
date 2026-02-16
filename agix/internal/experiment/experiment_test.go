package experiment

import (
	"testing"
)

func TestNew_NilWhenEmpty(t *testing.T) {
	m := New(nil)
	if m != nil {
		t.Error("expected nil for empty experiments")
	}
}

func TestNew_NilWhenAllDisabled(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: false, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 50},
	})
	if m != nil {
		t.Error("expected nil when all experiments disabled")
	}
}

func TestAssign_NoMatch(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 50},
	})
	a := m.Assign("agent-1", "claude-sonnet-4-20250514")
	if a != nil {
		t.Error("expected nil for non-matching model")
	}
}

func TestAssign_Consistency(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 50},
	})

	// Same agent+experiment should always get same variant
	a1 := m.Assign("agent-1", "gpt-4o")
	a2 := m.Assign("agent-1", "gpt-4o")

	if a1.Variant != a2.Variant {
		t.Error("assignment should be consistent for same agent")
	}
	if a1.ExperimentName != "test" {
		t.Errorf("ExperimentName = %q, want %q", a1.ExperimentName, "test")
	}
}

func TestAssign_ValidVariants(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 50},
	})

	// Test multiple agents to verify both variants are possible
	controlCount := 0
	variantCount := 0
	for i := 0; i < 100; i++ {
		agent := "agent-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		a := m.Assign(agent, "gpt-4o")
		if a == nil {
			t.Fatal("expected assignment")
		}
		switch a.Variant {
		case "control":
			controlCount++
			if a.Model != "gpt-4o" {
				t.Errorf("control model = %q, want gpt-4o", a.Model)
			}
		case "variant":
			variantCount++
			if a.Model != "gpt-4o-mini" {
				t.Errorf("variant model = %q, want gpt-4o-mini", a.Model)
			}
		default:
			t.Errorf("unexpected variant: %q", a.Variant)
		}
	}

	// With 50% traffic, we expect roughly half in each
	if controlCount == 0 || variantCount == 0 {
		t.Errorf("expected both variants to appear: control=%d, variant=%d", controlCount, variantCount)
	}
}

func TestAssign_ZeroTraffic(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 0},
	})

	// All should be control
	for i := 0; i < 20; i++ {
		a := m.Assign("agent-"+string(rune('a'+i)), "gpt-4o")
		if a.Variant != "control" {
			t.Error("expected all control with 0% traffic")
		}
	}
}

func TestAssign_FullTraffic(t *testing.T) {
	m := New([]Config{
		{Name: "test", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 100},
	})

	// All should be variant
	for i := 0; i < 20; i++ {
		a := m.Assign("agent-"+string(rune('a'+i)), "gpt-4o")
		if a.Variant != "variant" {
			t.Error("expected all variant with 100% traffic")
		}
	}
}

func TestList(t *testing.T) {
	m := New([]Config{
		{Name: "exp1", Enabled: true, ControlModel: "gpt-4o", VariantModel: "gpt-4o-mini", TrafficPct: 50},
		{Name: "exp2", Enabled: false, ControlModel: "claude-sonnet-4-20250514", VariantModel: "claude-haiku-4-5-20251001", TrafficPct: 20},
		{Name: "exp3", Enabled: true, ControlModel: "gpt-5", VariantModel: "gpt-4o", TrafficPct: 10},
	})

	list := m.List()
	if len(list) != 2 {
		t.Errorf("len = %d, want 2 (only enabled)", len(list))
	}
}

func TestHashBucket_Range(t *testing.T) {
	for i := 0; i < 1000; i++ {
		agent := "agent-" + string(rune(i%256))
		b := hashBucket(agent, "test")
		if b < 0 || b >= 100 {
			t.Errorf("bucket %d out of range [0, 100)", b)
		}
	}
}
