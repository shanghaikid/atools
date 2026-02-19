package promptinject

import (
	"encoding/json"
	"log"
	"strings"
)

// Config holds prompt template injection settings.
type Config struct {
	Global   string
	Agents   map[string]string
	Position string // "prepend" or "append", default "prepend"
}

// Injector injects system prompts into chat completion requests.
type Injector struct {
	global   string
	agents   map[string]string
	position string
}

// New creates a new Injector. Returns nil if no templates are configured.
func New(cfg Config) *Injector {
	if cfg.Global == "" && len(cfg.Agents) == 0 {
		return nil
	}
	pos := cfg.Position
	if pos != "append" {
		pos = "prepend"
	}
	return &Injector{
		global:   cfg.Global,
		agents:   cfg.Agents,
		position: pos,
	}
}

// Inject injects the effective system prompt into the request body's messages array.
// Operates on OpenAI-compatible format (before Anthropic conversion).
func (inj *Injector) Inject(body []byte, agentName string) []byte {
	prompt := inj.effectivePrompt(agentName)
	if prompt == "" {
		return body
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	messagesRaw, ok := raw["messages"]
	if !ok {
		return body
	}

	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body
	}

	if len(messages) > 0 && hasRole(messages[0], "system") {
		// Existing system message — prepend or append to its content
		var content string
		if err := json.Unmarshal(messages[0]["content"], &content); err != nil {
			return body
		}
		if inj.position == "prepend" {
			content = prompt + "\n" + content
		} else {
			content = content + "\n" + prompt
		}
		contentJSON, _ := json.Marshal(content)
		messages[0]["content"] = contentJSON
	} else {
		// No system message — insert one at position 0
		contentJSON, _ := json.Marshal(prompt)
		sysMsg := map[string]json.RawMessage{
			"role":    json.RawMessage(`"system"`),
			"content": contentJSON,
		}
		messages = append([]map[string]json.RawMessage{sysMsg}, messages...)
	}

	newMessages, err := json.Marshal(messages)
	if err != nil {
		return body
	}
	raw["messages"] = newMessages

	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}

	log.Printf("INJECT: system prompt injected for agent %q (%d chars)", agentName, len(prompt))
	return out
}

// effectivePrompt builds the effective prompt for the given agent.
func (inj *Injector) effectivePrompt(agentName string) string {
	var parts []string
	if inj.global != "" {
		parts = append(parts, inj.global)
	}
	if agentName != "" && inj.agents != nil {
		if agentPrompt, ok := inj.agents[agentName]; ok && agentPrompt != "" {
			parts = append(parts, agentPrompt)
		}
	}
	return strings.Join(parts, "\n")
}

// hasRole checks if a message has the given role.
func hasRole(msg map[string]json.RawMessage, role string) bool {
	roleRaw, ok := msg["role"]
	if !ok {
		return false
	}
	var r string
	if err := json.Unmarshal(roleRaw, &r); err != nil {
		return false
	}
	return r == role
}
