package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// mockServer simulates an MCP server using io.Pipe.
type mockServer struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newMockServer() (*mockServer, io.Reader, io.WriteCloser) {
	// Client writes to serverR, reads from clientR
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()

	return &mockServer{r: serverR, w: serverW}, clientR, clientW
}

func (s *mockServer) readRequest() (jsonRPCRequest, error) {
	buf := make([]byte, 64*1024)
	n, err := s.r.Read(buf)
	if err != nil {
		return jsonRPCRequest{}, err
	}
	var req jsonRPCRequest
	if err := json.Unmarshal(buf[:n], &req); err != nil {
		return jsonRPCRequest{}, err
	}
	return req, nil
}

func (s *mockServer) sendResponse(id any, result any) error {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
	}
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resp.Result = data
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "%s\n", data)
	return err
}

func (s *mockServer) sendError(id any, code int, message string) error {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "%s\n", data)
	return err
}

func (s *mockServer) close() {
	s.w.Close()
	s.r.Close()
}

// handleInitialize reads the initialize request and responds, then reads the initialized notification.
func (s *mockServer) handleInitialize(t *testing.T) {
	t.Helper()
	req, err := s.readRequest()
	if err != nil {
		t.Fatalf("read initialize request: %v", err)
	}
	if req.Method != "initialize" {
		t.Fatalf("expected initialize, got %s", req.Method)
	}
	s.sendResponse(req.ID, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"serverInfo":      map[string]any{"name": "test-server", "version": "1.0.0"},
	})

	// Read initialized notification
	notif, err := s.readRequest()
	if err != nil {
		t.Fatalf("read initialized notification: %v", err)
	}
	if notif.Method != "notifications/initialized" {
		t.Fatalf("expected notifications/initialized, got %s", notif.Method)
	}
}

func TestClientInitialize(t *testing.T) {
	server, clientR, clientW := newMockServer()
	defer server.close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		server.handleInitialize(t)
	}()

	client := NewClientFromIO("test", clientR, clientW)
	if err := client.initialize(); err != nil {
		t.Fatalf("initialize error: %v", err)
	}

	<-done

	if client.Name() != "test" {
		t.Errorf("Name() = %q, want %q", client.Name(), "test")
	}
}

func TestClientListTools(t *testing.T) {
	server, clientR, clientW := newMockServer()
	defer server.close()

	client := NewClientFromIO("test", clientR, clientW)

	done := make(chan struct{})
	go func() {
		defer close(done)
		req, err := server.readRequest()
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if req.Method != "tools/list" {
			t.Errorf("method = %q, want tools/list", req.Method)
		}
		server.sendResponse(req.ID, map[string]any{
			"tools": []map[string]any{
				{
					"name":        "read_file",
					"description": "Read a file from disk",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path": map[string]any{"type": "string"},
						},
					},
				},
				{
					"name":        "write_file",
					"description": "Write a file to disk",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path":    map[string]any{"type": "string"},
							"content": map[string]any{"type": "string"},
						},
					},
				},
			},
		})
	}()

	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}
	<-done

	if len(tools) != 2 {
		t.Fatalf("ListTools() returned %d tools, want 2", len(tools))
	}
	if tools[0].Name != "read_file" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "read_file")
	}
	if tools[1].Name != "write_file" {
		t.Errorf("tools[1].Name = %q, want %q", tools[1].Name, "write_file")
	}
	if tools[0].Description != "Read a file from disk" {
		t.Errorf("tools[0].Description = %q, want %q", tools[0].Description, "Read a file from disk")
	}
}

func TestClientCallTool(t *testing.T) {
	server, clientR, clientW := newMockServer()
	defer server.close()

	client := NewClientFromIO("test", clientR, clientW)

	done := make(chan struct{})
	go func() {
		defer close(done)
		req, err := server.readRequest()
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if req.Method != "tools/call" {
			t.Errorf("method = %q, want tools/call", req.Method)
		}
		// Verify params
		params, _ := json.Marshal(req.Params)
		var p map[string]any
		json.Unmarshal(params, &p)
		if p["name"] != "read_file" {
			t.Errorf("tool name = %v, want read_file", p["name"])
		}

		server.sendResponse(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "file contents here"},
			},
		})
	}()

	result, err := client.CallTool("read_file", map[string]any{"path": "/tmp/test.txt"})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	<-done

	if len(result.Content) != 1 {
		t.Fatalf("result.Content len = %d, want 1", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want %q", result.Content[0].Type, "text")
	}
	if result.Content[0].Text != "file contents here" {
		t.Errorf("content text = %q, want %q", result.Content[0].Text, "file contents here")
	}
}

func TestClientCallToolError(t *testing.T) {
	server, clientR, clientW := newMockServer()
	defer server.close()

	client := NewClientFromIO("test", clientR, clientW)

	done := make(chan struct{})
	go func() {
		defer close(done)
		req, _ := server.readRequest()
		server.sendError(req.ID, -32600, "invalid tool")
	}()

	_, err := client.CallTool("nonexistent", nil)
	<-done

	if err == nil {
		t.Fatal("CallTool() expected error, got nil")
	}
}

func TestClientCallToolIsError(t *testing.T) {
	server, clientR, clientW := newMockServer()
	defer server.close()

	client := NewClientFromIO("test", clientR, clientW)

	done := make(chan struct{})
	go func() {
		defer close(done)
		req, _ := server.readRequest()
		server.sendResponse(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "file not found"},
			},
			"isError": true,
		})
	}()

	result, err := client.CallTool("read_file", map[string]any{"path": "/nonexistent"})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	<-done

	if !result.IsError {
		t.Error("expected IsError=true")
	}
	if result.Content[0].Text != "file not found" {
		t.Errorf("error text = %q, want %q", result.Content[0].Text, "file not found")
	}
}
