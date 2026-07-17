// Package mcp serves gw's workspace capabilities as a Model Context Protocol
// (MCP) stdio server, so an agent can inspect and drive a Go workspace directly:
// list modules, lint version drift, map the dependency graph, compute the
// change-affected set, sync go.work, and run tests across every module.
//
// The transport is newline-delimited JSON-RPC 2.0 over stdin/stdout, per the MCP
// stdio spec: one JSON object per line, no embedded newlines.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const protocolVersion = "2025-06-18"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// server holds the wiring for one stdio session.
type server struct {
	out     io.Writer
	enc     *json.Encoder
	version string
	tools   []Tool
	byName  map[string]Tool
}

// Serve runs the MCP protocol loop until in is exhausted. version is reported in
// the initialize handshake. It returns any fatal transport error (EOF is nil).
func Serve(in io.Reader, out io.Writer, version string) error {
	s := &server{out: out, enc: json.NewEncoder(out), version: version}
	s.tools = allTools()
	s.byName = make(map[string]Tool, len(s.tools))
	for _, t := range s.tools {
		s.byName[t.Name] = t
	}

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // allow large tool payloads
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.replyError(nil, -32700, "parse error: "+err.Error())
			continue
		}
		s.handle(req)
	}
	return sc.Err()
}

func (s *server) handle(req request) {
	switch req.Method {
	case "initialize":
		s.reply(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "gw", "version": s.version},
		})
	case "notifications/initialized", "notifications/cancelled":
		// notifications carry no id and expect no response
	case "ping":
		s.reply(req.ID, map[string]any{})
	case "tools/list":
		descs := make([]toolDesc, len(s.tools))
		for i, t := range s.tools {
			descs[i] = toolDesc{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
		}
		s.reply(req.ID, map[string]any{"tools": descs})
	case "tools/call":
		s.callTool(req)
	default:
		if len(req.ID) > 0 {
			s.replyError(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

type toolDesc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

func (s *server) callTool(req request) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.replyError(req.ID, -32602, "invalid params: "+err.Error())
		return
	}
	t, ok := s.byName[p.Name]
	if !ok {
		s.replyError(req.ID, -32602, "unknown tool: "+p.Name)
		return
	}
	text, err := t.Handler(p.Arguments)
	if err != nil {
		// Tool-level errors are returned as a result with isError, per MCP, so the
		// agent can read the message rather than the call hard-failing.
		s.reply(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
		return
	}
	s.reply(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	})
}

func (s *server) reply(id json.RawMessage, result any) {
	if len(id) == 0 {
		return // notification: no response
	}
	_ = s.enc.Encode(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *server) replyError(id json.RawMessage, code int, msg string) {
	if len(id) == 0 {
		fmt.Fprintln(s.out, `{"jsonrpc":"2.0","id":null,"error":{"code":`+itoa(code)+`,"message":`+quote(msg)+`}}`)
		return
	}
	_ = s.enc.Encode(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
