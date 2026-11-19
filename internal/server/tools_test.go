package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

func callReq(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("empty result")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", res.Content[0])
	}
	return tc.Text
}

// zuulStub records the last request body and path sent to the fake Zuul API.
type zuulStub struct {
	path string
	body map[string]any
	hits int
}

func newTestServer(t *testing.T, token string) (*Server, *zuulStub) {
	t.Helper()
	stub := &zuulStub{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.hits++
		stub.path = r.URL.EscapedPath()
		stub.body = nil
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&stub.body)
		}
		fmt.Fprint(w, `true`)
	}))
	t.Cleanup(ts.Close)
	cfg := &config.Config{ZuulURL: ts.URL, DefaultTenant: "t", AuthToken: token}
	return New(cfg), stub
}

func TestRegisteredTools(t *testing.T) {
	s, _ := newTestServer(t, "")
	tools := s.MCPServer().ListTools()
	if _, ok := tools["get_job_variants"]; ok {
		t.Errorf("get_job_variants targets a non-existent Zuul endpoint and must not be registered")
	}
	for _, name := range []string{"get_job", "create_autohold", "promote", "enqueue", "dequeue"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
	promote := tools["promote"].Tool.InputSchema.Properties["changes"].(map[string]any)
	if promote["type"] != "array" {
		t.Errorf("promote.changes must be an array, got %v", promote["type"])
	}
}

func TestChangeList(t *testing.T) {
	tests := []struct {
		name    string
		in      any
		want    []string
		wantErr bool
	}{
		{"array keeps commas", []any{"12345,1", " 13336,3 "}, []string{"12345,1", "13336,3"}, false},
		{"whitespace string", "12345,1 13336,3", []string{"12345,1", "13336,3"}, false},
		{"empty array", []any{}, nil, true},
		{"non-string item", []any{float64(1)}, nil, true},
		{"blank item", []any{"12345,1", " "}, nil, true},
		{"missing", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changeList(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoholdID(t *testing.T) {
	tests := []struct {
		in      any
		want    string
		wantErr bool
	}{
		{"0000000042", "0000000042", false},
		{float64(42), "0000000042", false},
		{"42", "0000000042", false},
		{float64(1.5), "", true},
		{float64(-1), "", true},
		{float64(9007199254740993), "", true},
		{"  ", "", true},
		{"../x", "", true},
		{"12345678901", "", true},
		{true, "", true},
		{nil, "", true},
	}
	for _, tt := range tests {
		got, err := autoholdID(callReq(map[string]any{"request_id": tt.in}))
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("autoholdID(%v) = %q, %v; want %q, wantErr %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestHandleCreateAutohold(t *testing.T) {
	s, stub := newTestServer(t, "tok")

	res, err := s.handleCreateAutohold(context.Background(), callReq(map[string]any{
		"project":              "org/repo",
		"job":                  "tox",
		"reason":               "debug",
		"change":               "12345",
		"count":                float64(2),
		"node_hold_expiration": float64(3600),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	if stub.path != "/api/tenant/t/project/org/repo/autohold" {
		t.Errorf("unexpected path %s", stub.path)
	}
	want := map[string]any{
		"job": "tox", "reason": "debug", "count": float64(2),
		"change": "12345", "ref": nil, "node_hold_expiration": float64(3600),
	}
	if !reflect.DeepEqual(stub.body, want) {
		t.Errorf("body = %v, want %v", stub.body, want)
	}
}

func TestHandleCreateAutohold_Validation(t *testing.T) {
	base := map[string]any{"project": "p", "job": "j", "reason": "r"}
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name  string
		args  map[string]any
		token string
		want  string
	}{
		{"no auth", base, "", "authentication"},
		{"change and ref", with(map[string]any{"change": "1", "ref": "refs/heads/main"}), "tok", "mutually exclusive"},
		{"zero count", with(map[string]any{"count": float64(0)}), "tok", "'count' must be an integer"},
		{"fractional count", with(map[string]any{"count": float64(1.5)}), "tok", "'count' must be an integer"},
		{"huge count", with(map[string]any{"count": float64(1e20)}), "tok", "'count' must be an integer"},
		{"garbage count", with(map[string]any{"count": "garbage"}), "tok", "'count' must be an integer"},
		{"bool count", with(map[string]any{"count": true}), "tok", "'count' must be an integer"},
		{"negative expiration", with(map[string]any{"node_hold_expiration": float64(-1)}), "tok", "'node_hold_expiration' must be an integer"},
		{"numeric change", with(map[string]any{"change": float64(12345)}), "tok", "'change' must be a string"},
		{"legacy ref_filter", with(map[string]any{"ref_filter": "refs/heads/main"}), "tok", "use 'ref'"},
		{"legacy node_expiration", with(map[string]any{"node_expiration": float64(60)}), "tok", "use 'node_hold_expiration'"},
		{"blank ref", with(map[string]any{"ref": " "}), "tok", "'ref' must not be blank"},
		{"blank change", with(map[string]any{"change": "\t"}), "tok", "'change' must not be blank"},
		{"empty ref", with(map[string]any{"ref": ""}), "tok", "'ref' must not be blank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, stub := newTestServer(t, tt.token)
			res, err := s.handleCreateAutohold(context.Background(), callReq(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.IsError || !strings.Contains(resultText(t, res), tt.want) {
				t.Errorf("expected error containing %q, got %q", tt.want, resultText(t, res))
			}
			if stub.hits != 0 {
				t.Errorf("Zuul must not be called on validation failure")
			}
		})
	}
}

func TestHandlePromote_SendsChangeArray(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, err := s.handlePromote(context.Background(), callReq(map[string]any{
		"pipeline": "gate",
		"changes":  []any{"12345,1", "13336,3"},
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	if got := stub.body["changes"]; !reflect.DeepEqual(got, []any{"12345,1", "13336,3"}) {
		t.Errorf("changes = %v", got)
	}
}

func TestHandleEnqueue_Validation(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"neither", map[string]any{}, "must be provided"},
		{"both", map[string]any{"change": "1,1", "ref": "refs/heads/main"}, "not both"},
		{"rev with change", map[string]any{"change": "1,1", "newrev": "abc"}, "only be used with 'ref'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, stub := newTestServer(t, "tok")
			tt.args["project"] = "p"
			tt.args["pipeline"] = "check"
			res, _ := s.handleEnqueue(context.Background(), callReq(tt.args))
			if !res.IsError || !strings.Contains(resultText(t, res), tt.want) {
				t.Errorf("expected error containing %q, got %q", tt.want, resultText(t, res))
			}
			if stub.hits != 0 {
				t.Errorf("Zuul must not be called on validation failure")
			}
		})
	}
}

func TestHandleCreateAutohold_StringCountAndNullFilters(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, err := s.handleCreateAutohold(context.Background(), callReq(map[string]any{
		"project": "p", "job": "j", "reason": "r",
		"count": "3", "change": nil, "node_hold_expiration": nil,
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	if stub.body["count"] != float64(3) || stub.body["node_hold_expiration"] != nil || stub.body["change"] != nil {
		t.Errorf("unexpected body: %v", stub.body)
	}
}

func TestHandleCreateAutohold_RefRegexVerbatim(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, err := s.handleCreateAutohold(context.Background(), callReq(map[string]any{
		"project": "p", "job": "j", "reason": "r", "ref": "^refs/heads/main$ ",
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	if got := stub.body["ref"]; got != "^refs/heads/main$ " {
		t.Errorf("ref = %q, want it sent verbatim", got)
	}
}

func TestHandleEnqueue_RejectsNonStringChange(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, _ := s.handleEnqueue(context.Background(), callReq(map[string]any{
		"project": "p", "pipeline": "check", "change": float64(12345),
	}))
	if !res.IsError || !strings.Contains(resultText(t, res), "'change' must be a string") {
		t.Errorf("expected type error, got %q", resultText(t, res))
	}
	if stub.hits != 0 {
		t.Errorf("Zuul must not be called on validation failure")
	}
}

func TestHandleEnqueue_PassesRefAndRevs(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, err := s.handleEnqueue(context.Background(), callReq(map[string]any{
		"project": "org/repo", "pipeline": "post",
		"ref": "refs/heads/main", "oldrev": "aaa", "newrev": "bbb",
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	if stub.path != "/api/tenant/t/project/org/repo/enqueue" {
		t.Errorf("path = %s", stub.path)
	}
	want := map[string]any{"pipeline": "post", "ref": "refs/heads/main", "oldrev": "aaa", "newrev": "bbb"}
	if !reflect.DeepEqual(stub.body, want) {
		t.Errorf("body = %v, want %v", stub.body, want)
	}
}

func TestHandleDequeue_PassesChange(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, err := s.handleDequeue(context.Background(), callReq(map[string]any{
		"project": "org/repo", "pipeline": "gate", "change": "12345,1",
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: %v %s", err, resultText(t, res))
	}
	want := map[string]any{"pipeline": "gate", "change": "12345,1"}
	if stub.path != "/api/tenant/t/project/org/repo/dequeue" || !reflect.DeepEqual(stub.body, want) {
		t.Errorf("path = %s body = %v, want body %v", stub.path, stub.body, want)
	}
}

func TestRequiredStringsRejectBlank(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, _ := s.handleCreateAutohold(context.Background(), callReq(map[string]any{
		"project": "p", "job": " ", "reason": "r",
	}))
	if !res.IsError || !strings.Contains(resultText(t, res), "'job' must not be blank") {
		t.Errorf("expected blank job error, got %q", resultText(t, res))
	}
	res, _ = s.handlePromote(context.Background(), callReq(map[string]any{
		"pipeline": "", "changes": []any{"1,1"},
	}))
	if !res.IsError || !strings.Contains(resultText(t, res), "'pipeline' must not be blank") {
		t.Errorf("expected blank pipeline error, got %q", resultText(t, res))
	}
	if stub.hits != 0 {
		t.Errorf("Zuul must not be called on validation failure")
	}
}

func TestHandleDequeue_RejectsBoth(t *testing.T) {
	s, stub := newTestServer(t, "tok")
	res, _ := s.handleDequeue(context.Background(), callReq(map[string]any{
		"project": "p", "pipeline": "gate", "change": "1,1", "ref": "refs/heads/main",
	}))
	if !res.IsError {
		t.Errorf("expected error when both change and ref are set")
	}
	if stub.hits != 0 {
		t.Errorf("Zuul must not be called on validation failure")
	}
}

func newLogServer(t *testing.T, log string) *Server {
	t.Helper()
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/job-output.txt") {
			fmt.Fprint(w, log)
			return
		}
		fmt.Fprintf(w, `{"uuid":"b","log_url":%q}`, ts.URL+"/logs/b/")
	}))
	t.Cleanup(ts.Close)
	return New(&config.Config{ZuulURL: ts.URL, DefaultTenant: "t"})
}

func TestLastLines(t *testing.T) {
	tests := []struct {
		in   string
		n    int64
		want string
		cut  bool
	}{
		{"a\nb\nc\n", 2, "b\nc\n", true},
		{"a\nb\nc", 2, "b\nc", true},
		{"a\nb\nc\n", 3, "a\nb\nc\n", false},
		{"a\nb\nc\n", 10, "a\nb\nc\n", false},
		{"a\n\n\n", 1, "\n", true},
		{"", 1, "", false},
		{"\n", 1, "\n", false},
	}
	for _, tt := range tests {
		got, cut := lastLines(tt.in, tt.n)
		if got != tt.want || cut != tt.cut {
			t.Errorf("lastLines(%q, %d) = %q, %v; want %q, %v", tt.in, tt.n, got, cut, tt.want, tt.cut)
		}
	}
}

func TestHandleGetBuildLogs(t *testing.T) {
	s := newLogServer(t, "one\ntwo\nthree\n")

	res, _ := s.handleGetBuildLogs(context.Background(), callReq(map[string]any{"uuid": "b"}))
	if got := resultText(t, res); got != "one\ntwo\nthree\n" || res.IsError {
		t.Errorf("full log = %q", got)
	}

	res, _ = s.handleGetBuildLogs(context.Background(), callReq(map[string]any{"uuid": "b", "tail_lines": 2.0}))
	got := resultText(t, res)
	if !strings.HasSuffix(got, "]\ntwo\nthree\n") || !strings.Contains(got, "full log: http") {
		t.Errorf("tail_lines output = %q", got)
	}

	res, _ = s.handleGetBuildLogs(context.Background(), callReq(map[string]any{"uuid": "b", "max_bytes": 6.0}))
	got = resultText(t, res)
	if !strings.HasSuffix(got, "]\nthree\n") {
		t.Errorf("max_bytes output = %q", got)
	}

	res, _ = s.handleGetBuildLogs(context.Background(), callReq(map[string]any{"uuid": "b", "tail_lines": 5.0}))
	if got := resultText(t, res); got != "one\ntwo\nthree\n" {
		t.Errorf("tail_lines larger than the log = %q", got)
	}

	for _, args := range []map[string]any{
		{"uuid": "b", "tail_lines": -1.0},
		{"uuid": "b", "max_bytes": 0.0},
		{"uuid": "b", "max_bytes": 2e6},
		{"uuid": "b", "max_bytes": "lots"},
	} {
		res, _ := s.handleGetBuildLogs(context.Background(), callReq(args))
		if !res.IsError {
			t.Errorf("%v: expected error, got %q", args, resultText(t, res))
		}
	}
}
