package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
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

func TestGetJobVariants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tenant/test-tenant/job/my-job/variants" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"name":"my-job","branches":["main"]},{"name":"my-job","branches":["stable"]}]`)
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	variants, err := c.GetJobVariants(context.Background(), "test-tenant", "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
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
		w.WriteHeader(http.StatusOK)
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
		w.WriteHeader(http.StatusOK)
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
		w.WriteHeader(http.StatusOK)
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

func TestListComponents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/components" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"schedulers": []map[string]any{
				{"hostname": "scheduler-1", "state": "running", "version": "9.0.0", "process_id": 100},
			},
			"executors": []map[string]any{
				{"hostname": "executor-1", "state": "running", "version": "9.0.0", "process_id": 200, "accepting_work": true},
				{"hostname": "executor-2", "state": "running", "version": "9.0.0", "process_id": 201, "accepting_work": false},
			},
			"mergers": []map[string]any{
				{"hostname": "merger-1", "state": "running", "version": "9.0.0", "process_id": 300},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Config{ZuulURL: server.URL}
	c := New(cfg)

	components, err := c.ListComponents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(components.Schedulers) != 1 {
		t.Errorf("expected 1 scheduler, got %d", len(components.Schedulers))
	}
	if components.Schedulers[0].Hostname != "scheduler-1" {
		t.Errorf("expected scheduler-1, got %s", components.Schedulers[0].Hostname)
	}
	if components.Schedulers[0].State != "running" {
		t.Errorf("expected state running, got %s", components.Schedulers[0].State)
	}

	if len(components.Executors) != 2 {
		t.Errorf("expected 2 executors, got %d", len(components.Executors))
	}
	if !components.Executors[0].AcceptingWork {
		t.Errorf("expected executor-1 to be accepting work")
	}
	if components.Executors[1].AcceptingWork {
		t.Errorf("expected executor-2 to not be accepting work")
	}

	if len(components.Mergers) != 1 {
		t.Errorf("expected 1 merger, got %d", len(components.Mergers))
	}

	if len(components.Fingergateways) != 0 {
		t.Errorf("expected 0 fingergateways, got %d", len(components.Fingergateways))
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
