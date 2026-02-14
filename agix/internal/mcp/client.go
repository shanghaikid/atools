package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a piece of content in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError is a JSON-RPC 2.0 error object.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client is an MCP client that communicates with an MCP server over stdio.
type Client struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	nextID  atomic.Int64
}

// NewClient spawns an MCP server process and performs the initialize handshake.
func NewClient(name, command string, args []string, env []string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr

	// Set environment variables
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: create stdin pipe: %w", name, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: create stdout pipe: %w", name, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: start process: %w", name, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
	}

	if err := c.initialize(); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// NewClientFromIO creates a Client from existing reader/writer (for testing).
func NewClientFromIO(name string, r io.Reader, w io.WriteCloser) *Client {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	return &Client{
		name:    name,
		stdin:   w,
		scanner: scanner,
	}
}

// Name returns the server name.
func (c *Client) Name() string {
	return c.name
}

func (c *Client) initialize() error {
	// Send initialize request
	resp, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "agix",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("mcp %s: initialize: %w", c.name, err)
	}
	_ = resp // We don't need to inspect the result for now

	// Send initialized notification (no ID, no response expected)
	if err := c.notify("notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp %s: send initialized notification: %w", c.name, err)
	}

	return nil
}

// ListTools returns all tools available on this MCP server.
func (c *Client) ListTools() ([]Tool, error) {
	resp, err := c.call("tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp %s: tools/list: %w", c.name, err)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp %s: parse tools/list response: %w", c.name, err)
	}

	return result.Tools, nil
}

// CallTool executes a tool on the MCP server.
func (c *Client) CallTool(name string, arguments map[string]any) (*ToolResult, error) {
	params := map[string]any{
		"name": name,
	}
	if arguments != nil {
		params["arguments"] = arguments
	}

	resp, err := c.call("tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcp %s: tools/call %s: %w", c.name, name, err)
	}

	var result ToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp %s: parse tools/call response: %w", c.name, err)
	}

	return &result, nil
}

// Close shuts down the MCP server process.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stdin.Close()

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Signal(os.Interrupt)
		return c.cmd.Wait()
	}
	return nil
}

// call sends a JSON-RPC request and waits for the response.
func (c *Client) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response lines, skipping notifications
	for {
		if !c.scanner.Scan() {
			if err := c.scanner.Err(); err != nil {
				return nil, fmt.Errorf("read response: %w", err)
			}
			return nil, fmt.Errorf("unexpected EOF reading response")
		}

		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // Skip non-JSON lines
		}

		// Skip notifications (no ID)
		if resp.ID == nil {
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		return resp.Result, nil
	}
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}
