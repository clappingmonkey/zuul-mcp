package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// readStub is a fake Zuul API that answers every GET with the body
// registered for its path (default "[]") and records the requests it saw.
type readStub struct {
	mu       sync.Mutex
	requests []*url.URL
	bodies   map[string]string
	status   int
}

func (z *readStub) last() *url.URL {
	z.mu.Lock()
	defer z.mu.Unlock()
	if len(z.requests) == 0 {
		return nil
	}
	return z.requests[len(z.requests)-1]
}

func (z *readStub) count() int {
	z.mu.Lock()
	defer z.mu.Unlock()
	return len(z.requests)
}

func newReadServer(t *testing.T, defaultTenant string, bodies map[string]string) (*Server, *readStub) {
	t.Helper()
	stub := &readStub{bodies: bodies, status: http.StatusOK}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.mu.Lock()
		u := *r.URL
		stub.requests = append(stub.requests, &u)
		status := stub.status
		stub.mu.Unlock()
		if r.Method != http.MethodGet {
			t.Errorf("read tool sent %s %s", r.Method, r.URL)
		}
		if status != http.StatusOK {
			http.Error(w, "boom", status)
			return
		}
		body, ok := bodies[r.URL.EscapedPath()]
		if !ok {
			body = "[]"
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(ts.Close)
	cfg := &config.Config{ZuulURL: ts.URL, DefaultTenant: defaultTenant}
	return New(cfg, "test"), stub
}

func callTool(t *testing.T, s *Server, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	tool, ok := s.MCPServer().ListTools()[name]
	if !ok {
		t.Fatalf("tool %q is not registered", name)
	}
	req := callReq(args)
	req.Params.Name = name
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s returned a protocol error: %v", name, err)
	}
	return res
}

// readToolCases lists every read-only tool with valid arguments and the
// Zuul request it must produce.
var readToolCases = []struct {
	tool      string
	args      map[string]any
	path      string // escaped path
	query     url.Values
	global    bool // not tenant-scoped
	wantInOut string
	wantOut   string // exact result text
}{
	{tool: "list_tenants", path: "/api/tenants", global: true},
	{tool: "list_connections", path: "/api/connections", global: true},
	{tool: "list_components", path: "/api/components", global: true, wantInOut: "scheduler"},
	{
		tool: "list_builds",
		args: map[string]any{"project": "org/repo", "pipeline": "check", "branch": "main", "result": "FAILURE", "job_name": "lint", "change": 12345.0, "limit": 5.0, "skip": 10.0},
		path: "/api/tenant/t/builds",
		query: url.Values{
			"project": {"org/repo"}, "pipeline": {"check"}, "branch": {"main"}, "result": {"FAILURE"},
			"job_name": {"lint"}, "change": {"12345"}, "limit": {"5"}, "skip": {"10"},
		},
	},
	{tool: "list_builds", path: "/api/tenant/t/builds", query: url.Values{"limit": {"50"}}},
	{tool: "get_build", args: map[string]any{"uuid": "abc"}, path: "/api/tenant/t/build/abc", wantInOut: `"uuid":"abc"`},
	{
		tool:  "list_buildsets",
		args:  map[string]any{"project": "org/repo", "result": "SUCCESS", "change": "7", "limit": 2.0},
		path:  "/api/tenant/t/buildsets",
		query: url.Values{"project": {"org/repo"}, "result": {"SUCCESS"}, "change": {"7"}, "limit": {"2"}},
	},
	{tool: "get_buildset", args: map[string]any{"uuid": "bs1"}, path: "/api/tenant/t/buildset/bs1", wantInOut: `"uuid":"bs1"`},
	{tool: "list_jobs", path: "/api/tenant/t/jobs"},
	{tool: "get_job", args: map[string]any{"job_name": "org/my+job"}, path: "/api/tenant/t/job/org%2Fmy%252Bjob"},
	{tool: "list_pipelines", path: "/api/tenant/t/pipelines"},
	{tool: "get_pipeline_status", args: map[string]any{"pipeline": "gate"}, path: "/api/tenant/t/status", wantOut: `{"name":"gate","change_queues":[]}`},
	{tool: "list_projects", path: "/api/tenant/t/projects"},
	{tool: "get_project", args: map[string]any{"project": "org/repo"}, path: "/api/tenant/t/project/org/repo"},
	{tool: "get_tenant_status", path: "/api/tenant/t/status", wantInOut: "pipelines"},
	{tool: "get_change_status", args: map[string]any{"change": "12345,1"}, path: "/api/tenant/t/status/change/12345%2C1"},
	{tool: "get_config_errors", path: "/api/tenant/t/config-errors", query: url.Values{"limit": {"50"}}},
	{
		tool:  "get_config_errors",
		args:  map[string]any{"project": "org/repo", "branch": "main", "severity": "warning", "name": "Job Not Found", "limit": 10.0, "skip": 20.0},
		path:  "/api/tenant/t/config-errors",
		query: url.Values{"project": {"org/repo"}, "branch": {"main"}, "severity": {"warning"}, "name": {"Job Not Found"}, "limit": {"10"}, "skip": {"20"}},
	},
	{tool: "list_system_events", path: "/api/tenant/t/system-events"},
	{
		tool:  "list_system_events",
		args:  map[string]any{"event_type": "reconfiguration", "limit": 3.0},
		path:  "/api/tenant/t/system-events",
		query: url.Values{"event_type": {"reconfiguration"}, "limit": {"3"}},
	},
	{tool: "list_nodes", path: "/api/tenant/t/nodes"},
	{tool: "list_labels", path: "/api/tenant/t/labels"},
	{tool: "list_semaphores", path: "/api/tenant/t/semaphores"},
	{tool: "list_autoholds", path: "/api/tenant/t/autohold"},
	{tool: "get_autohold", args: map[string]any{"request_id": "0000000042"}, path: "/api/tenant/t/autohold/0000000042", wantInOut: "0000000042"},
	{tool: "get_build_logs", args: map[string]any{"uuid": "abc"}, path: "/api/tenant/t/build/abc"},
}

var readFixtures = map[string]string{
	"/api/components":                   `{"scheduler":[{"hostname":"zs1","state":"running","version":"12.0.0"}]}`,
	"/api/tenant/t/build/abc":           `{"uuid":"abc","job_name":"lint"}`,
	"/api/tenant/t/buildset/bs1":        `{"uuid":"bs1"}`,
	"/api/tenant/t/status":              `{"pipelines":[{"name":"check"},{"name":"gate","change_queues":[]}]}`,
	"/api/tenant/t/project/org/repo":    `{"name":"org/repo"}`,
	"/api/tenant/t/autohold/0000000042": `{"id":"0000000042","job":"lint"}`,
}

func TestReadTools_RequestAndResult(t *testing.T) {
	s, stub := newReadServer(t, "t", readFixtures)
	for _, tc := range readToolCases {
		name := tc.tool
		if len(tc.args) == 0 {
			name += "_defaults"
		}
		t.Run(name, func(t *testing.T) {
			res := callTool(t, s, tc.tool, tc.args)
			if tc.tool == "get_build_logs" {
				// The build has no log_url: the tool reports that cleanly.
				if !res.IsError || !strings.Contains(resultText(t, res), "no log URL") {
					t.Fatalf("get_build_logs = %q", resultText(t, res))
				}
			} else if res.IsError {
				t.Fatalf("%s failed: %s", tc.tool, resultText(t, res))
			}
			got := stub.last()
			if got == nil {
				t.Fatal("no request reached Zuul")
			}
			if got.EscapedPath() != tc.path {
				t.Errorf("path = %s, want %s", got.EscapedPath(), tc.path)
			}
			if want := tc.query; want == nil && got.RawQuery != "" {
				t.Errorf("query = %q, want none", got.RawQuery)
			} else if want != nil && got.Query().Encode() != want.Encode() {
				t.Errorf("query = %q, want %q", got.Query().Encode(), want.Encode())
			}
			if tc.wantOut != "" && resultText(t, res) != tc.wantOut {
				t.Errorf("result = %q, want %q", resultText(t, res), tc.wantOut)
			}
			if tc.wantInOut != "" && !strings.Contains(resultText(t, res), tc.wantInOut) {
				t.Errorf("result %q does not contain %q", resultText(t, res), tc.wantInOut)
			}
		})
	}
}

func TestReadTools_ExplicitTenant(t *testing.T) {
	s, stub := newReadServer(t, "t", nil)
	res := callTool(t, s, "list_jobs", map[string]any{"tenant": "other"})
	if res.IsError {
		t.Fatal(resultText(t, res))
	}
	if got := stub.last().EscapedPath(); got != "/api/tenant/other/jobs" {
		t.Errorf("path = %s", got)
	}
}

func TestReadTools_TenantRequired(t *testing.T) {
	s, stub := newReadServer(t, "", nil)
	for _, tc := range readToolCases {
		if tc.global {
			continue
		}
		res := callTool(t, s, tc.tool, tc.args)
		if !res.IsError || !strings.Contains(resultText(t, res), "tenant") {
			t.Errorf("%s without a tenant = %q, want a tenant error", tc.tool, resultText(t, res))
		}
	}
	if n := stub.count(); n != 0 {
		t.Errorf("%d requests reached Zuul without a tenant", n)
	}
}

func TestReadTools_RejectMistypedArguments(t *testing.T) {
	s, stub := newReadServer(t, "t", nil)
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"list_jobs", map[string]any{"tenant": 42.0}},
		{"list_builds", map[string]any{"project": 1.0}},
		{"list_builds", map[string]any{"result": true}},
		{"list_buildsets", map[string]any{"branch": []any{"main"}}},
		{"list_system_events", map[string]any{"event_type": 3.0}},
		{"get_build", map[string]any{"uuid": " "}},
		{"get_job", map[string]any{}},
		{"get_config_errors", map[string]any{"severity": "fatal"}},
		{"get_config_errors", map[string]any{"limit": 0.0}},
		{"get_config_errors", map[string]any{"name": 1.0}},
	} {
		if res := callTool(t, s, tc.tool, tc.args); !res.IsError {
			t.Errorf("%s(%v) succeeded, want an argument error", tc.tool, tc.args)
		}
	}
	if n := stub.count(); n != 0 {
		t.Errorf("%d requests reached Zuul with invalid arguments", n)
	}
}

func TestReadTools_BlankFiltersAreIgnored(t *testing.T) {
	s, stub := newReadServer(t, "t", nil)
	res := callTool(t, s, "list_builds", map[string]any{"tenant": "", "project": " ", "result": nil})
	if res.IsError {
		t.Fatal(resultText(t, res))
	}
	if got := stub.last(); got.EscapedPath() != "/api/tenant/t/builds" || got.RawQuery != "limit=50" {
		t.Errorf("request = %s?%s", got.EscapedPath(), got.RawQuery)
	}
}

func TestReadTools_UpstreamErrors(t *testing.T) {
	s, stub := newReadServer(t, "t", nil)
	stub.mu.Lock()
	stub.status = http.StatusInternalServerError
	stub.mu.Unlock()
	for _, tc := range readToolCases {
		res := callTool(t, s, tc.tool, tc.args)
		if !res.IsError {
			t.Errorf("%s succeeded on a Zuul 500: %s", tc.tool, resultText(t, res))
		}
	}
}

func TestGetPipelineStatus_NotFound(t *testing.T) {
	s, _ := newReadServer(t, "t", readFixtures)
	res := callTool(t, s, "get_pipeline_status", map[string]any{"pipeline": "post"})
	if !res.IsError || !strings.Contains(resultText(t, res), "post") {
		t.Errorf("result = %q, want a pipeline-not-found error", resultText(t, res))
	}
}

func TestReadTools_EmptyListsAreArrays(t *testing.T) {
	s, _ := newReadServer(t, "t", map[string]string{"/api/tenant/t/jobs": "null"})
	if got := resultText(t, callTool(t, s, "list_jobs", nil)); got != "[]" {
		t.Errorf("list_jobs = %q, want []", got)
	}
}
