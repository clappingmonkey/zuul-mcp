package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

var writeTools = []string{"create_autohold", "delete_autohold", "enqueue", "dequeue", "promote"}

func toolNames(s *Server) []string {
	var names []string
	for name := range s.MCPServer().ListTools() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestToolCatalogMatchesRegistration(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test", AuthToken: "token"}, "test")
	tools := s.MCPServer().ListTools()
	if len(tools) != len(toolCatalog) {
		t.Errorf("registered %d tools, catalog has %d", len(tools), len(toolCatalog))
	}
	for name := range toolCatalog {
		if _, ok := tools[name]; !ok {
			t.Errorf("catalog entry %q is not registered", name)
		}
	}
}

func TestToolAnnotations(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test", AuthToken: "token"}, "test")
	isWrite := map[string]bool{}
	for _, n := range writeTools {
		isWrite[n] = true
	}
	for name, st := range s.MCPServer().ListTools() {
		a := st.Tool.Annotations
		if a.Title == "" {
			t.Errorf("%s: missing title", name)
		}
		if a.ReadOnlyHint == nil || a.DestructiveHint == nil || a.IdempotentHint == nil || a.OpenWorldHint == nil {
			t.Errorf("%s: all hints must be set explicitly, got %+v", name, a)
			continue
		}
		if *a.ReadOnlyHint == isWrite[name] {
			t.Errorf("%s: readOnlyHint=%v, want %v", name, *a.ReadOnlyHint, !isWrite[name])
		}
		if *a.ReadOnlyHint && (*a.DestructiveHint || !*a.IdempotentHint) {
			t.Errorf("%s: read-only tools must be non-destructive and idempotent", name)
		}
		if !*a.OpenWorldHint {
			t.Errorf("%s: every tool talks to Zuul and is open-world", name)
		}
	}
	for _, name := range []string{"delete_autohold", "dequeue", "promote"} {
		if a := s.MCPServer().ListTools()[name].Tool.Annotations; !*a.DestructiveHint {
			t.Errorf("%s must be marked destructive", name)
		}
	}
	for _, name := range []string{"create_autohold", "enqueue"} {
		if a := s.MCPServer().ListTools()[name].Tool.Annotations; *a.DestructiveHint {
			t.Errorf("%s only adds work and must not be marked destructive", name)
		}
	}
}

func TestAnnotationsSerialized(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test"}, "test")
	data, err := json.Marshal(s.MCPServer().ListTools()["list_builds"].Tool)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"readOnlyHint":true`, `"destructiveHint":false`, `"title":"List builds"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("serialized tool missing %s: %s", want, data)
		}
	}
}

func TestWriteToolsHidden(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.Config
		wantWrite bool
	}{
		{"no token", config.Config{ZuulURL: "http://zuul.test"}, false},
		{"token", config.Config{ZuulURL: "http://zuul.test", AuthToken: "token"}, true},
		{"token but read-only", config.Config{ZuulURL: "http://zuul.test", AuthToken: "token", ReadOnly: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			s := New(&cfg, "test")
			tools := s.MCPServer().ListTools()
			for _, name := range writeTools {
				if _, ok := tools[name]; ok != tt.wantWrite {
					t.Errorf("%s registered=%v, want %v (tools: %v)", name, ok, tt.wantWrite, toolNames(s))
				}
			}
			if _, ok := tools["list_builds"]; !ok {
				t.Error("read tools must always be registered")
			}
			if got, want := len(tools), len(toolCatalog)-len(writeTools); !tt.wantWrite && got != want {
				t.Errorf("got %d tools, want %d", got, want)
			}
		})
	}
}

func TestHiddenWriteToolsAreNotCallable(t *testing.T) {
	s, stub := newTestServer(t, "")
	for i, name := range writeTools {
		msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":{"project":"org/repo","pipeline":"check","change":"1,1","changes":["1,1"],"job":"j","reason":"r","request_id":"0000000001"}}}`, i, name)
		raw := s.MCPServer().HandleMessage(context.Background(), []byte(msg))
		resp, ok := raw.(mcp.JSONRPCError)
		if !ok {
			t.Errorf("%s: expected a JSON-RPC error, got %T: %+v", name, raw, raw)
			continue
		}
		if resp.Error.Code != mcp.INVALID_PARAMS {
			t.Errorf("%s: error code %d, want %d", name, resp.Error.Code, mcp.INVALID_PARAMS)
		}
	}
	if stub.hits != 0 {
		t.Errorf("hidden write tools reached Zuul %d times", stub.hits)
	}
}

func schemaProperty(t *testing.T, s *Server, tool, prop string) map[string]any {
	t.Helper()
	st, ok := s.MCPServer().ListTools()[tool]
	if !ok {
		t.Fatalf("tool %s not registered", tool)
	}
	data, err := json.Marshal(st.Tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	p, ok := schema.Properties[prop]
	if !ok {
		t.Fatalf("%s has no property %s", tool, prop)
	}
	return p
}

func TestNumericSchemas(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test", AuthToken: "token"}, "test")
	for name, st := range s.MCPServer().ListTools() {
		for prop := range st.Tool.InputSchema.Properties {
			if typ := schemaProperty(t, s, name, prop)["type"]; typ == "number" {
				t.Errorf("%s.%s: every numeric argument is an integer, schema says %q", name, prop, typ)
			}
		}
	}

	maxInt32 := float64(math.MaxInt32)
	tests := []struct {
		tool, prop string
		min, max   float64
		def        any
	}{
		{"list_builds", "change", 0, maxInt32, nil},
		{"list_builds", "limit", 1, 1000, float64(defaultListLimit)},
		{"list_builds", "skip", 0, maxInt32, nil},
		{"list_buildsets", "change", 0, maxInt32, nil},
		{"list_buildsets", "limit", 1, 1000, float64(defaultListLimit)},
		{"list_buildsets", "skip", 0, maxInt32, nil},
		{"get_config_errors", "limit", 1, 1000, float64(defaultListLimit)},
		{"get_config_errors", "skip", 0, maxInt32, nil},
		{"list_system_events", "limit", 1, 1000, nil},
		{"list_system_events", "skip", 0, maxInt32, nil},
		{"get_build_logs", "tail_lines", 0, maxInt32, nil},
		{"get_build_logs", "max_bytes", 1, maxLogBytes, float64(defaultLogBytes)},
		{"create_autohold", "count", 1, maxInt32, float64(1)},
		{"create_autohold", "node_hold_expiration", 0, maxInt32, nil},
	}
	for _, tt := range tests {
		p := schemaProperty(t, s, tt.tool, tt.prop)
		if p["type"] != "integer" || p["minimum"] != tt.min || p["maximum"] != tt.max || p["default"] != tt.def {
			t.Errorf("%s.%s schema = %v, want integer [%v, %v] default %v", tt.tool, tt.prop, p, tt.min, tt.max, tt.def)
		}
	}
}

func TestConfigErrorsSchema(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test"}, "test")
	for _, prop := range []string{"tenant", "project", "branch", "severity", "name"} {
		if typ := schemaProperty(t, s, "get_config_errors", prop)["type"]; typ != "string" {
			t.Errorf("get_config_errors.%s type = %v, want string", prop, typ)
		}
	}
	enum, _ := schemaProperty(t, s, "get_config_errors", "severity")["enum"].([]any)
	if len(enum) != 2 || enum[0] != "error" || enum[1] != "warning" {
		t.Errorf("get_config_errors.severity enum = %v, want [error warning]", enum)
	}
}

func TestInitializeReportsVersionAndInstructions(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want []string
		not  []string
	}{
		{
			name: "read-only with default tenant",
			cfg:  config.Config{ZuulURL: "http://zuul.test", DefaultTenant: "openstack"},
			want: []string{`defaults to "openstack"`, "read-only", "ZUUL_AUTH_TOKEN is not set"},
			not:  []string{"confirm with the user"},
		},
		{
			name: "token but read-only mode",
			cfg:  config.Config{ZuulURL: "http://zuul.test", AuthToken: "token", ReadOnly: true},
			want: []string{"read-only mode is enabled"},
			not:  []string{"confirm with the user", "ZUUL_AUTH_TOKEN is not set"},
		},
		{
			name: "writes enabled without default tenant",
			cfg:  config.Config{ZuulURL: "http://zuul.test", AuthToken: "token"},
			want: []string{"need a tenant argument", "confirm with the user"},
			not:  []string{"read-only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			s := New(&cfg, "1.2.3")
			raw := s.MCPServer().HandleMessage(context.Background(), []byte(
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
			resp, ok := raw.(mcp.JSONRPCResponse)
			if !ok {
				t.Fatalf("unexpected response %T: %+v", raw, raw)
			}
			res, ok := resp.Result.(mcp.InitializeResult)
			if !ok {
				t.Fatalf("unexpected result %T", resp.Result)
			}
			if res.ServerInfo.Name != "zuul-mcp" || res.ServerInfo.Version != "1.2.3" {
				t.Errorf("serverInfo = %+v", res.ServerInfo)
			}
			for _, w := range tt.want {
				if !strings.Contains(res.Instructions, w) {
					t.Errorf("instructions missing %q:\n%s", w, res.Instructions)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(res.Instructions, n) {
					t.Errorf("instructions must not contain %q:\n%s", n, res.Instructions)
				}
			}
		})
	}
}

func TestAddToolPanicsWithoutCatalogEntry(t *testing.T) {
	s := New(&config.Config{ZuulURL: "http://zuul.test"}, "test")
	defer func() {
		if recover() == nil {
			t.Error("expected panic for an unannotated tool")
		}
	}()
	s.addTool(mcp.NewTool("not_in_catalog"), nil)
}
