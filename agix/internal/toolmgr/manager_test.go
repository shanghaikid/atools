package toolmgr

import (
	"testing"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/mcp"
)

func testTools() []ToolEntry {
	return []ToolEntry{
		{Tool: mcp.Tool{Name: "read_file", Description: "Read a file"}, Server: "filesystem"},
		{Tool: mcp.Tool{Name: "write_file", Description: "Write a file"}, Server: "filesystem"},
		{Tool: mcp.Tool{Name: "list_directory", Description: "List directory"}, Server: "filesystem"},
		{Tool: mcp.Tool{Name: "delete_file", Description: "Delete a file"}, Server: "filesystem"},
		{Tool: mcp.Tool{Name: "search_code", Description: "Search code"}, Server: "github"},
	}
}

func TestToolsForAgentNoConfig(t *testing.T) {
	m := NewFromClients(nil, nil)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("unknown-agent")
	if len(tools) != 5 {
		t.Errorf("ToolsForAgent(unknown) = %d tools, want 5", len(tools))
	}
}

func TestToolsForAgentAllowList(t *testing.T) {
	agents := map[string]config.AgentTools{
		"code-reviewer": {Allow: []string{"read_file", "list_directory"}},
	}

	m := NewFromClients(nil, agents)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("code-reviewer")
	if len(tools) != 2 {
		t.Fatalf("ToolsForAgent(code-reviewer) = %d tools, want 2", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["read_file"] {
		t.Error("expected read_file in allowed tools")
	}
	if !names["list_directory"] {
		t.Error("expected list_directory in allowed tools")
	}
	if names["write_file"] {
		t.Error("write_file should not be in allowed tools")
	}
}

func TestToolsForAgentDenyList(t *testing.T) {
	agents := map[string]config.AgentTools{
		"docs-writer": {Deny: []string{"write_file", "delete_file"}},
	}

	m := NewFromClients(nil, agents)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("docs-writer")
	if len(tools) != 3 {
		t.Fatalf("ToolsForAgent(docs-writer) = %d tools, want 3", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if names["write_file"] {
		t.Error("write_file should be denied")
	}
	if names["delete_file"] {
		t.Error("delete_file should be denied")
	}
	if !names["read_file"] {
		t.Error("expected read_file in result")
	}
	if !names["list_directory"] {
		t.Error("expected list_directory in result")
	}
	if !names["search_code"] {
		t.Error("expected search_code in result")
	}
}

func TestToolsForAgentEmptyConfig(t *testing.T) {
	agents := map[string]config.AgentTools{
		"open-agent": {}, // empty allow and deny
	}

	m := NewFromClients(nil, agents)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("open-agent")
	if len(tools) != 5 {
		t.Errorf("ToolsForAgent(open-agent) = %d tools, want 5", len(tools))
	}
}

func TestToolsForAgentNoTools(t *testing.T) {
	m := NewFromClients(nil, nil)

	tools := m.ToolsForAgent("any-agent")
	if tools != nil {
		t.Errorf("ToolsForAgent with no tools = %v, want nil", tools)
	}
}

func TestAllTools(t *testing.T) {
	m := NewFromClients(nil, nil)
	m.SetTools(testTools())

	if m.ToolCount() != 5 {
		t.Errorf("ToolCount() = %d, want 5", m.ToolCount())
	}

	all := m.AllTools()
	if len(all) != 5 {
		t.Errorf("AllTools() = %d, want 5", len(all))
	}
}

func TestServerCount(t *testing.T) {
	clients := map[string]*mcp.Client{
		"fs":     nil,
		"github": nil,
	}
	m := NewFromClients(clients, nil)

	if m.ServerCount() != 2 {
		t.Errorf("ServerCount() = %d, want 2", m.ServerCount())
	}
}

func TestAllowPrecedenceOverDeny(t *testing.T) {
	// If both allow and deny are set, allow takes precedence
	agents := map[string]config.AgentTools{
		"confused-agent": {
			Allow: []string{"read_file"},
			Deny:  []string{"write_file"},
		},
	}

	m := NewFromClients(nil, agents)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("confused-agent")
	if len(tools) != 1 {
		t.Fatalf("ToolsForAgent(confused-agent) = %d tools, want 1 (allow takes precedence)", len(tools))
	}
	if tools[0].Name != "read_file" {
		t.Errorf("tool name = %q, want read_file", tools[0].Name)
	}
}

func TestAllowNonExistentTool(t *testing.T) {
	agents := map[string]config.AgentTools{
		"agent": {Allow: []string{"nonexistent_tool"}},
	}

	m := NewFromClients(nil, agents)
	m.SetTools(testTools())

	tools := m.ToolsForAgent("agent")
	if len(tools) != 0 {
		t.Errorf("ToolsForAgent with nonexistent allow = %d tools, want 0", len(tools))
	}
}
