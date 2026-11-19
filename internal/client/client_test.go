package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/models"
)

func TestListTenants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenants" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"name": "tenant1"},
			{"name": "tenant2"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	tenants, err := c.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(tenants))
	}
	if tenants[0].Name != "tenant1" {
		t.Errorf("expected tenant1, got %s", tenants[0].Name)
	}
}

func TestListBuilds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/builds" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check query parameters
		if r.URL.Query().Get("project") != "my-project" {
			t.Errorf("expected project=my-project, got %s", r.URL.Query().Get("project"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "build-1", "job_name": "test-job", "result": "SUCCESS"},
		})
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	builds, err := c.ListBuilds(context.Background(), "test-tenant", &BuildsQuery{
		Project: "my-project",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(builds) != 1 {
		t.Errorf("expected 1 build, got %d", len(builds))
	}
	if builds[0].UUID != "build-1" {
		t.Errorf("expected build-1, got %s", builds[0].UUID)
	}
}

func TestAuthHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	cfg := &config.Config{
		ZuulURL:   server.URL,
		AuthToken: "my-jwt-token",
	}
	c := New(cfg)

	_, _ = c.ListTenants(context.Background())

	expected := "Bearer my-jwt-token"
	if receivedAuth != expected {
		t.Errorf("expected %q, got %q", expected, receivedAuth)
	}
}

func TestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("tenant not found"))
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.ListTenants(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetBuildLogs(t *testing.T) {
	// Create a server that handles both build and log requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tenant/test-tenant/build/build-123":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"uuid":     "build-123",
				"job_name": "test-job",
				"project":  "my-project",
				"pipeline": "check",
				"voting":   true,
				"log_url":  "http://" + r.Host + "/logs/build-123",
			})
		case "/logs/build-123/job-output.txt":
			w.Write([]byte("Job started\nRunning tests...\nJob finished"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	logs, err := c.GetBuildLogs(context.Background(), "test-tenant", "build-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(logs, "Running tests") {
		t.Errorf("expected logs to contain 'Running tests', got: %s", logs)
	}
}

func TestGetBuildLogs_BuildNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("build not found"))
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.GetBuildLogs(context.Background(), "test-tenant", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetBuildLogs_NoLogURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid":     "build-123",
			"job_name": "test-job",
			"project":  "my-project",
			"pipeline": "check",
			"voting":   true,
			// No log_url
		})
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.GetBuildLogs(context.Background(), "test-tenant", "build-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no log URL") {
		t.Errorf("expected 'no log URL' error, got: %v", err)
	}
}

func TestGetBuildLogs_LogsNotAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/build/") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"uuid":     "build-123",
				"job_name": "test-job",
				"project":  "my-project",
				"pipeline": "check",
				"voting":   true,
				"log_url":  "http://" + r.Host + "/logs/build-123",
			})
		} else {
			// Logs endpoint returns 404
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.GetBuildLogs(context.Background(), "test-tenant", "build-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "logs not available") {
		t.Errorf("expected 'logs not available' error, got: %v", err)
	}
}

func TestGetBuildset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/buildset/bs-uuid-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid":     "bs-uuid-123",
			"project":  "my-project",
			"pipeline": "check",
			"result":   "SUCCESS",
		})
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	buildset, err := c.GetBuildset(context.Background(), "test-tenant", "bs-uuid-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buildset.UUID != "bs-uuid-123" {
		t.Errorf("expected UUID 'bs-uuid-123', got '%s'", buildset.UUID)
	}
	if buildset.Pipeline != "check" {
		t.Errorf("expected pipeline 'check', got '%s'", buildset.Pipeline)
	}
}

func TestListNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/nodes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"node-1","state":"ready"},{"id":"node-2","state":"in-use"}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	nodes, err := c.ListNodes(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestListLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/labels" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"ubuntu-jammy"},{"name":"centos-9"}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	labels, err := c.ListLabels(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(labels))
	}
}

func TestListConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/connections" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"github","driver":"github"},{"name":"gerrit","driver":"gerrit"}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	connections, err := c.ListConnections(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(connections))
	}
}

func TestListSemaphores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/semaphores" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"sem-1","global":false,"max":2},{"name":"sem-2","global":true,"max":1}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	semaphores, err := c.ListSemaphores(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(semaphores) != 2 {
		t.Errorf("expected 2 semaphores, got %d", len(semaphores))
	}
}

func TestGetJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Zuul routes /job/{job_name} without a path wildcard. Cheroot percent-decodes
		// the path (except %2F) and Zuul then unquote_plus()es the name, so slashes
		// are encoded once and '+' / '%' twice.
		if got, want := r.URL.EscapedPath(), "/api/tenant/test-tenant/job/org%2Fmy%252Bjob"; got != want {
			t.Errorf("unexpected escaped path: got %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"org/my+job","branches":["main"]},{"name":"org/my+job","branches":["stable"]}]`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	variants, err := c.GetJob(context.Background(), "test-tenant", "org/my+job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
	}
}

func TestGetProject_KeepsSlashes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/tenant/test-tenant/project/org/repo"; got != want {
			t.Errorf("unexpected escaped path: got %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"org/repo","canonical_name":"github.com/org/repo","connection_name":"github","type":"untrusted","configs":[]}`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	project, err := c.GetProject(context.Background(), "test-tenant", "org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Name != "org/repo" {
		t.Errorf("expected project org/repo, got %s", project.Name)
	}
}

func TestGetPipelineStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"zuul_version":"13.0.0","pipelines":[
			{"name":"check","change_queues":[{"name":"q","heads":[[{"id":"a1b2","enqueue_time":1700000000123,"refs":[{"id":"1,1","project":"org/repo"}],"jobs":[{"name":"tox","elapsed_time":1200,"remaining_time":null,"start_time":1700000001.5}]}]]}]},
			{"name":"gate","change_queues":[]}
		]}`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	status, err := c.GetPipelineStatus(context.Background(), "test-tenant", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var pipeline struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(status, &pipeline); err != nil {
		t.Fatalf("decoding pipeline: %v", err)
	}
	if pipeline.Name != "check" {
		t.Errorf("expected pipeline check, got %s", pipeline.Name)
	}
	if !strings.Contains(string(status), `"enqueue_time":1700000000123`) {
		t.Errorf("expected raw item fields to be preserved, got %s", status)
	}

	_, err = c.GetPipelineStatus(context.Background(), "test-tenant", "missing")
	if !errors.Is(err, ErrPipelineNotFound) {
		t.Errorf("expected ErrPipelineNotFound, got %v", err)
	}
}

func TestListAutoholds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/autohold" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no auth header without a token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"0000000001","tenant":"test-tenant","project":"org/repo","job":"tox","ref_filter":".*","max_count":1,"current_count":0,"reason":"debug","node_expiration":86400,"expired":null,"nodes":[]}]`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	autoholds, err := c.ListAutoholds(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(autoholds) != 1 || autoholds[0].ID != "0000000001" {
		t.Errorf("unexpected autoholds: %+v", autoholds)
	}
}

func TestGetAutohold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/autohold/0000000042" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "0000000042",
			"tenant": "test-tenant",
			"project": "my-project",
			"job": "my-job",
			"ref_filter": ".*",
			"max_count": 3,
			"current_count": 1,
			"reason": "debugging flaky test",
			"node_expiration": 3600,
			"expired": null,
			"nodes": [{"build": "abc123", "nodes": ["0000000100"]}]
		}`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	autohold, err := c.GetAutohold(context.Background(), "test-tenant", "0000000042")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if autohold.ID != "0000000042" {
		t.Errorf("expected id 0000000042, got %s", autohold.ID)
	}
	if autohold.Project != "my-project" {
		t.Errorf("expected project my-project, got %s", autohold.Project)
	}
	if autohold.Job != "my-job" {
		t.Errorf("expected job my-job, got %s", autohold.Job)
	}
	if autohold.MaxCount != 3 {
		t.Errorf("expected max_count 3, got %d", autohold.MaxCount)
	}
	if autohold.CurrentCount != 1 {
		t.Errorf("expected current_count 1, got %d", autohold.CurrentCount)
	}
	if autohold.NodeExpiration != 3600 {
		t.Errorf("expected node_expiration 3600, got %d", autohold.NodeExpiration)
	}
	if !strings.Contains(string(autohold.Nodes), `"build": "abc123"`) {
		t.Errorf("expected nodes to be preserved, got %s", autohold.Nodes)
	}
}

func TestGetAutohold_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	_, err := c.GetAutohold(context.Background(), "test-tenant", "0000000099")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "getting autohold") {
		t.Errorf("expected error to contain 'getting autohold', got: %v", err)
	}
}

func TestCreateAutohold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got, want := r.URL.EscapedPath(), "/api/tenant/test-tenant/project/org/repo/autohold"; got != want {
			t.Errorf("unexpected escaped path: got %s, want %s", got, want)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("expected bearer token, got %q", r.Header.Get("Authorization"))
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		// zuul-web returns 400 unless every one of these keys is present.
		for _, key := range []string{"job", "count", "reason", "node_hold_expiration", "change", "ref"} {
			if _, ok := body[key]; !ok {
				t.Errorf("missing required body key %q in %v", key, body)
			}
		}
		if body["job"] != "tox" || body["count"] != float64(2) || body["change"] != "12345" {
			t.Errorf("unexpected body: %v", body)
		}
		if body["ref"] != nil || body["node_hold_expiration"] != nil {
			t.Errorf("expected null ref and node_hold_expiration, got %v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `true`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL, AuthToken: "secret"})

	change := "12345"
	err := c.CreateAutohold(context.Background(), "test-tenant", "org/repo", &models.AutoholdRequest{
		Job:    "tox",
		Reason: "debug",
		Count:  2,
		Change: &change,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAutohold_AuthNotConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL, AuthToken: "secret"})

	err := c.CreateAutohold(context.Background(), "t", "p", &models.AutoholdRequest{Job: "j", Reason: "r", Count: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "authentication") {
		t.Errorf("expected 404 with auth hint in error, got %v", err)
	}
}

func TestDeleteAutohold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/tenant/test-tenant/autohold/0000000007" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL, AuthToken: "secret"})

	if err := c.DeleteAutohold(context.Background(), "test-tenant", "0000000007"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnqueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/tenant/test-tenant/project/my-project/enqueue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body EnqueueRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body.Pipeline != "gate" || body.Change != "12345,1" {
			t.Errorf("unexpected body: %+v", body)
		}
		fmt.Fprint(w, `true`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL, AuthToken: "tok"}
	c := New(cfg)

	err := c.Enqueue(context.Background(), "test-tenant", "my-project", &EnqueueRequest{
		Pipeline: "gate",
		Change:   "12345,1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDequeue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/tenant/test-tenant/project/my-project/dequeue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body DequeueRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body.Pipeline != "gate" || body.Change != "12345,1" {
			t.Errorf("unexpected body: %+v", body)
		}
		fmt.Fprint(w, `true`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL, AuthToken: "tok"}
	c := New(cfg)

	err := c.Dequeue(context.Background(), "test-tenant", "my-project", &DequeueRequest{
		Pipeline: "gate",
		Change:   "12345,1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/tenant/test-tenant/promote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body PromoteRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body.Pipeline != "gate" || len(body.Changes) != 2 {
			t.Errorf("unexpected body: %+v", body)
		}
		fmt.Fprint(w, `true`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL, AuthToken: "tok"}
	c := New(cfg)

	err := c.Promote(context.Background(), "test-tenant", &PromoteRequest{
		Pipeline: "gate",
		Changes:  []string{"12345,1", "13336,3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnqueue_Ref(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/tenant/test-tenant/project/org/repo/enqueue"; got != want {
			t.Errorf("unexpected escaped path: got %s, want %s", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if _, ok := body["change"]; ok {
			t.Errorf("change must be omitted for ref enqueue: %v", body)
		}
		if _, ok := body["trigger"]; ok {
			t.Errorf("deprecated trigger must not be sent: %v", body)
		}
		if body["ref"] != "refs/heads/main" || body["newrev"] != "abc" {
			t.Errorf("unexpected body: %v", body)
		}
		fmt.Fprint(w, `true`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL, AuthToken: "tok"})

	err := c.Enqueue(context.Background(), "test-tenant", "org/repo", &EnqueueRequest{
		Pipeline: "post",
		Ref:      "refs/heads/main",
		NewRev:   "abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListComponents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/components" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// zuul-web keys components by singular kind and only includes kinds
		// with at least one registered component.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"scheduler": [{"hostname": "scheduler-1", "state": "running", "version": "13.0.0"}],
			"executor": [
				{"hostname": "executor-1", "state": "running", "version": "13.0.0"},
				{"hostname": "executor-2", "state": "paused", "version": "13.0.0"}
			],
			"launcher": [{"hostname": "launcher-1", "state": "running", "version": "13.0.0"}]
		}`)
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})

	components, err := c.ListComponents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := components["scheduler"]; len(got) != 1 || got[0].Hostname != "scheduler-1" || got[0].State != "running" {
		t.Errorf("unexpected schedulers: %+v", got)
	}
	if got := components["executor"]; len(got) != 2 || got[1].State != "paused" {
		t.Errorf("unexpected executors: %+v", got)
	}
	if got := components["launcher"]; len(got) != 1 {
		t.Errorf("expected launcher to be preserved, got %+v", got)
	}
	if _, ok := components["merger"]; ok {
		t.Errorf("expected no merger entry")
	}
}

func TestListComponents_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.ListComponents(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing components") {
		t.Errorf("expected error to contain 'listing components', got: %v", err)
	}
}

func TestGetChangeStatus(t *testing.T) {
	// The change ID includes a comma (Gerrit format). net/http decodes percent-encoding
	// in r.URL.Path automatically, so the handler sees the decoded form "12345,1".
	// The raw (encoded) form is available in r.URL.RawPath if needed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/status/change/12345,1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		// Response is an array of pipeline status objects (opaque — not spec'd by Zuul).
		fmt.Fprint(w, `[{"name":"gate","change_queues":[]},{"name":"check","change_queues":[]}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	pipelines, err := c.GetChangeStatus(context.Background(), "test-tenant", "12345,1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipeline entries, got %d", len(pipelines))
	}
}

func TestGetChangeStatus_Empty(t *testing.T) {
	// Change not queued in any pipeline — Zuul returns an empty array.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	pipelines, err := c.GetChangeStatus(context.Background(), "test-tenant", "99999,1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 0 {
		t.Errorf("expected 0 pipeline entries, got %d", len(pipelines))
	}
}

func TestGetChangeStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("tenant not found"))
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.GetChangeStatus(context.Background(), "unknown-tenant", "12345,1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "getting change status") {
		t.Errorf("expected error to contain 'getting change status', got: %v", err)
	}
}

func TestListSystemEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/system-events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{
				"description": "Configuration updated",
				"event_id": "evt-001",
				"event_time": "2026-06-01T10:00:00",
				"event_type": "config_update"
			},
			{
				"description": "Reconfiguration triggered",
				"event_id": "evt-002",
				"event_time": "2026-06-01T11:00:00",
				"event_type": "reconfiguration"
			}
		]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	events, err := c.ListSystemEvents(context.Background(), "test-tenant", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	if events[0].EventID != "evt-001" {
		t.Errorf("expected evt-001, got %s", events[0].EventID)
	}
	if events[0].EventType != "config_update" {
		t.Errorf("expected config_update, got %s", events[0].EventType)
	}
	if events[0].EventTime == nil || events[0].EventTime.IsZero() {
		t.Errorf("expected non-zero event_time for evt-001")
	}
	if events[1].EventID != "evt-002" {
		t.Errorf("expected evt-002, got %s", events[1].EventID)
	}
}

func TestListSystemEvents_WithFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/system-events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("event_type") != "config_update" {
			t.Errorf("expected event_type=config_update, got %s", r.URL.Query().Get("event_type"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("skip") != "5" {
			t.Errorf("expected skip=5, got %s", r.URL.Query().Get("skip"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"description":"cfg updated","event_id":"e1","event_type":"config_update"}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	events, err := c.ListSystemEvents(context.Background(), "test-tenant", &SystemEventsQuery{
		EventType: "config_update",
		Limit:     10,
		Skip:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestListSystemEvents_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("tenant not found"))
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	_, err := c.ListSystemEvents(context.Background(), "unknown-tenant", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing system events") {
		t.Errorf("expected error to contain 'listing system events', got: %v", err)
	}
}

// zuulDecodeJobName mimics how zuul-web recovers a job name from the request
// path: cheroot percent-decodes everything except %2F, then the job handler
// applies Python's unquote_plus.
func zuulDecodeJobName(t *testing.T, wire string) string {
	t.Helper()
	parts := regexp.MustCompile(`(?i)%2F`).Split(wire, -1)
	for i, p := range parts {
		dec, err := url.PathUnescape(p)
		if err != nil {
			t.Fatalf("cheroot stage failed for %q: %v", wire, err)
		}
		parts[i] = dec
	}
	name, err := url.QueryUnescape(strings.Join(parts, "%2F"))
	if err != nil {
		t.Fatalf("unquote_plus stage failed for %q: %v", wire, err)
	}
	return name
}

func TestJobPath_RoundTrip(t *testing.T) {
	// "a@b/c" -> "a@b%2Fc" mirrors zuul's own test (test_web.py: job/a%40b%2Fc).
	for _, name := range []string{"plain-job", "a@b/c", "org/my+job", "50%off", "a b", "a%2Fb", "x?y#z", "ümlaut"} {
		wire := jobPath(name)
		if strings.ContainsAny(wire, "/?#") {
			t.Errorf("jobPath(%q) = %q contains a raw path delimiter", name, wire)
		}
		if got := zuulDecodeJobName(t, wire); got != name {
			t.Errorf("jobPath(%q) = %q decodes to %q", name, wire, got)
		}
	}
}

func TestSend_Acknowledgement(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		status  int
		body    string
		wantErr bool
	}{
		{"post true", http.MethodPost, 200, "true\n", false},
		{"post false", http.MethodPost, 200, "false", true},
		{"post html login page", http.MethodPost, 200, "<html>Sign in</html>", true},
		{"post empty 200", http.MethodPost, 200, "", true},
		{"post 204 without ack", http.MethodPost, 204, "", true},
		{"post oversized with true prefix", http.MethodPost, 200, "true" + strings.Repeat(" ", maxErrorBody) + "false", true},
		{"post true padded to limit", http.MethodPost, 200, "true" + strings.Repeat(" ", maxErrorBody-4), false},
		{"delete 204", http.MethodDelete, 204, "", false},
		{"delete 200 empty", http.MethodDelete, 200, "", true},
		{"delete 200 html login page", http.MethodDelete, 200, "<html>Sign in</html>", true},
		{"delete 200 false", http.MethodDelete, 200, "false", true},
		{"post 403", http.MethodPost, 403, "forbidden", true},
		{"post 302 to login", http.MethodPost, 302, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			c := NewWithHTTPClient(&config.Config{ZuulURL: server.URL, AuthToken: "tok"}, hc)
			err := c.send(context.Background(), tt.method, "/x", map[string]string{"a": "b"})
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSend_TruncatedAcknowledgement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "true")
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL, AuthToken: "tok"})
	if err := c.send(context.Background(), http.MethodPost, "/x", nil); err == nil {
		t.Fatal("expected error for a response body cut short")
	}
}

func TestErrorBodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, strings.Repeat("x", 64*1024))
	}))
	defer server.Close()

	c := New(&config.Config{ZuulURL: server.URL})
	_, err := c.ListTenants(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) > maxErrorBody+200 || !strings.Contains(err.Error(), "(truncated)") {
		t.Errorf("error body not bounded (len %d)", len(err.Error()))
	}
}
