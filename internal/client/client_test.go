package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
