package experiment

import (
	"fmt"
	"hash/fnv"
)

// Config defines an A/B test experiment.
type Config struct {
	Name         string `yaml:"name"`
	Enabled      bool   `yaml:"enabled"`
	ControlModel string `yaml:"control_model"`
	VariantModel string `yaml:"variant_model"`
	TrafficPct   int    `yaml:"traffic_pct"` // 0-100, percentage routed to variant
}

// Assignment is the result of experiment evaluation.
type Assignment struct {
	ExperimentName string
	Variant        string // "control" or "variant"
	Model          string
}

// Manager evaluates experiment assignments.
type Manager struct {
	experiments []Config
}

// New creates an experiment Manager. Returns nil if no experiments are enabled.
func New(experiments []Config) *Manager {
	var enabled []Config
	for _, e := range experiments {
		if e.Enabled {
			enabled = append(enabled, e)
		}
	}
	if len(enabled) == 0 {
		return nil
	}
	return &Manager{experiments: enabled}
}

// Assign determines which experiment variant an agent should use for a given model.
// Uses FNV-1a consistent hashing so the same agent always gets the same variant.
// Returns nil if no experiment matches the model.
func (m *Manager) Assign(agentName, model string) *Assignment {
	for _, exp := range m.experiments {
		if exp.ControlModel != model {
			continue
		}

		bucket := hashBucket(agentName, exp.Name)
		if bucket < exp.TrafficPct {
			return &Assignment{
				ExperimentName: exp.Name,
				Variant:        "variant",
				Model:          exp.VariantModel,
			}
		}
		return &Assignment{
			ExperimentName: exp.Name,
			Variant:        "control",
			Model:          exp.ControlModel,
		}
	}
	return nil
}

// List returns all enabled experiments.
func (m *Manager) List() []Config {
	return m.experiments
}

// hashBucket returns a consistent 0-99 bucket for the given agent+experiment.
func hashBucket(agentName, experimentName string) int {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%s", agentName, experimentName)
	return int(h.Sum32() % 100)
}
