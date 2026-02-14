package toolmgr

import (
	"fmt"
	"log"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/mcp"
)

// ToolEntry is a tool with its owning server name.
type ToolEntry struct {
	mcp.Tool
	Server string // which MCP server provides this tool
}

// Manager aggregates tools from multiple MCP servers and handles per-agent filtering.
type Manager struct {
	clients map[string]*mcp.Client // server name → client
	tools   []ToolEntry            // all discovered tools
	agents  map[string]config.AgentTools
}

// New creates a Manager, connecting to all configured MCP servers.
func New(cfg config.ToolsConfig) (*Manager, error) {
	m := &Manager{
		clients: make(map[string]*mcp.Client),
		agents:  cfg.Agents,
	}

	for name, srv := range cfg.Servers {
		client, err := mcp.NewClient(name, srv.Command, srv.Args, srv.Env)
		if err != nil {
			// Close any already-started clients
			m.Close()
			return nil, fmt.Errorf("start MCP server %q: %w", name, err)
		}
		m.clients[name] = client

		tools, err := client.ListTools()
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("list tools from %q: %w", name, err)
		}
		for _, t := range tools {
			m.tools = append(m.tools, ToolEntry{Tool: t, Server: name})
		}
	}

	return m, nil
}

// NewFromClients creates a Manager from pre-built clients (for testing).
func NewFromClients(clients map[string]*mcp.Client, agents map[string]config.AgentTools) *Manager {
	return &Manager{
		clients: clients,
		agents:  agents,
	}
}

// SetTools sets the tool list directly (for testing).
func (m *Manager) SetTools(tools []ToolEntry) {
	m.tools = tools
}

// AllTools returns all discovered tools.
func (m *Manager) AllTools() []ToolEntry {
	return m.tools
}

// ServerCount returns the number of connected MCP servers.
func (m *Manager) ServerCount() int {
	return len(m.clients)
}

// ToolCount returns the total number of discovered tools.
func (m *Manager) ToolCount() int {
	return len(m.tools)
}

// ToolsForAgent returns the filtered list of tools available to a given agent.
// If the agent has no configuration, all tools are returned.
func (m *Manager) ToolsForAgent(agentName string) []ToolEntry {
	if len(m.tools) == 0 {
		return nil
	}

	agentCfg, ok := m.agents[agentName]
	if !ok {
		// No config for this agent → all tools
		return m.tools
	}

	if len(agentCfg.Allow) > 0 {
		return m.filterAllow(agentCfg.Allow)
	}

	if len(agentCfg.Deny) > 0 {
		return m.filterDeny(agentCfg.Deny)
	}

	// Empty config → all tools
	return m.tools
}

func (m *Manager) filterAllow(allow []string) []ToolEntry {
	set := make(map[string]bool, len(allow))
	for _, name := range allow {
		set[name] = true
	}
	var result []ToolEntry
	for _, t := range m.tools {
		if set[t.Name] {
			result = append(result, t)
		}
	}
	return result
}

func (m *Manager) filterDeny(deny []string) []ToolEntry {
	set := make(map[string]bool, len(deny))
	for _, name := range deny {
		set[name] = true
	}
	var result []ToolEntry
	for _, t := range m.tools {
		if !set[t.Name] {
			result = append(result, t)
		}
	}
	return result
}

// CallTool routes a tool call to the correct MCP server and executes it.
func (m *Manager) CallTool(toolName string, arguments map[string]any) (string, error) {
	// Find which server owns this tool
	var entry *ToolEntry
	for i := range m.tools {
		if m.tools[i].Name == toolName {
			entry = &m.tools[i]
			break
		}
	}
	if entry == nil {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	client, ok := m.clients[entry.Server]
	if !ok {
		return "", fmt.Errorf("no client for server %q", entry.Server)
	}

	result, err := client.CallTool(toolName, arguments)
	if err != nil {
		return "", err
	}

	// Concatenate text content blocks
	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	if result.IsError {
		return text, fmt.Errorf("tool error: %s", text)
	}

	return text, nil
}

// Close shuts down all MCP server processes.
func (m *Manager) Close() {
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			log.Printf("WARN: close MCP server %q: %v", name, err)
		}
	}
}
