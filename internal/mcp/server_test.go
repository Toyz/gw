package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// runSession feeds newline-delimited JSON-RPC lines through Serve and returns
// the decoded response objects (notifications produce none).
func runSession(t *testing.T, lines ...string) []map[string]any {
	t.Helper()
	var out strings.Builder
	if err := Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, "test"); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var msgs []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad response line %q: %v", l, err)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func TestInitializeAndToolsList(t *testing.T) {
	msgs := runSession(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // no response
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 responses (notification is silent), got %d", len(msgs))
	}
	init := msgs[0]["result"].(map[string]any)
	if init["protocolVersion"] != protocolVersion {
		t.Errorf("protocolVersion = %v", init["protocolVersion"])
	}
	tools := msgs[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	names := map[string]bool{}
	for _, tv := range tools {
		names[tv.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"gw_list", "gw_lint", "gw_graph", "gw_affected", "gw_doctor"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	msgs := runSession(t,
		`{"jsonrpc":"2.0","id":1,"method":"bogus/method"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	)
	if e := msgs[0]["error"].(map[string]any); e["code"].(float64) != -32601 {
		t.Errorf("unknown method code = %v", e["code"])
	}
	if e := msgs[1]["error"].(map[string]any); e["code"].(float64) != -32602 {
		t.Errorf("unknown tool code = %v", e["code"])
	}
}
