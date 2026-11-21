package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
)

// fixtureServer answers a single GET path with body and fails the test on any
// other request. It returns the client and a pointer to the last request URL.
func fixtureServer(t *testing.T, path, body string) (*Client, *url.URL) {
	t.Helper()
	seen := &url.URL{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r.URL
		if r.Method != http.MethodGet || r.URL.EscapedPath() != path {
			t.Errorf("unexpected request %s %s, want GET %s", r.Method, r.URL.EscapedPath(), path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(ts.Close)
	return New(&config.Config{ZuulURL: ts.URL}), seen
}

func errorServer(t *testing.T) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)
	return New(&config.Config{ZuulURL: ts.URL})
}

func TestListBuildsets(t *testing.T) {
	c, seen := fixtureServer(t, "/api/tenant/t/buildsets", `[
		{"uuid":"bs1","result":"SUCCESS","pipeline":"gate","event_id":"e1",
		 "first_build_start_time":"2026-11-18T10:00:00","last_build_end_time":"2026-11-18T10:05:00",
		 "refs":[{"project":"org/repo","change":12345,"patchset":"2"}]}
	]`)
	buildsets, err := c.ListBuildsets(context.Background(), "t", &BuildsetsQuery{
		Project: "org/repo", Pipeline: "gate", Change: 12345, Branch: "main", Ref: "refs/heads/main",
		Result: "SUCCESS", Limit: 5, Skip: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"project": {"org/repo"}, "pipeline": {"gate"}, "change": {"12345"}, "branch": {"main"},
		"ref": {"refs/heads/main"}, "result": {"SUCCESS"}, "limit": {"5"}, "skip": {"10"},
	}
	if got := seen.Query().Encode(); got != want.Encode() {
		t.Errorf("query = %q, want %q", got, want.Encode())
	}
	if len(buildsets) != 1 || buildsets[0].UUID != "bs1" || buildsets[0].Pipeline != "gate" {
		t.Fatalf("buildsets = %+v", buildsets)
	}
	if refs := buildsets[0].Refs; len(refs) != 1 || refs[0].Project != "org/repo" || refs[0].Change != 12345 || refs[0].Patchset != "2" {
		t.Errorf("refs = %+v", refs)
	}
	if bs := buildsets[0]; bs.FirstBuild == nil || bs.FirstBuild.Hour() != 10 || bs.LastBuild == nil || bs.LastBuild.Minute() != 5 {
		t.Errorf("naive timestamps not parsed: %+v", bs)
	}
}

func TestListBuildsets_NoQuery(t *testing.T) {
	c, seen := fixtureServer(t, "/api/tenant/t/buildsets", `[]`)
	buildsets, err := c.ListBuildsets(context.Background(), "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	if seen.RawQuery != "" || len(buildsets) != 0 {
		t.Errorf("query = %q, buildsets = %v", seen.RawQuery, buildsets)
	}
}

func TestListJobs(t *testing.T) {
	c, _ := fixtureServer(t, "/api/tenant/t/jobs", `[
		{"name":"base","description":"Base job","tags":["base"],"variants":[{"parent":null,"branches":[]}]},
		{"name":"lint","variants":[{"parent":"base","branches":["main","stable"]}]}
	]`)
	jobs, err := c.ListJobs(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].Name != "base" || jobs[0].Description != "Base job" || len(jobs[0].Tags) != 1 || jobs[0].Tags[0] != "base" {
		t.Fatalf("jobs = %+v", jobs)
	}
	if v := jobs[1].Variants; len(v) != 1 || v[0].Parent != "base" || len(v[0].Branches) != 2 {
		t.Errorf("variants = %+v", v)
	}
}

func TestListPipelines(t *testing.T) {
	c, _ := fixtureServer(t, "/api/tenant/t/pipelines", `[
		{"name":"check","triggers":[{"name":"gerrit","driver":"gerrit"}]},
		{"name":"periodic","triggers":[{"name":"timer","driver":"timer"}]}
	]`)
	pipelines, err := c.ListPipelines(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines) != 2 || pipelines[0].Name != "check" {
		t.Fatalf("pipelines = %+v", pipelines)
	}
	if tr := pipelines[1].Triggers; len(tr) != 1 || tr[0].Name != "timer" || tr[0].Driver != "timer" {
		t.Errorf("triggers = %+v", tr)
	}
}

func TestListProjects(t *testing.T) {
	c, _ := fixtureServer(t, "/api/tenant/t/projects", `[
		{"name":"org/config","connection_name":"gerrit","canonical_name":"review.example.com/org/config","type":"config"},
		{"name":"org/repo","connection_name":"gerrit","canonical_name":"review.example.com/org/repo","type":"untrusted"}
	]`)
	projects, err := c.ListProjects(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].Name != "org/config" || projects[0].ConnectionName != "gerrit" ||
		projects[0].Type != "config" || projects[1].Name != "org/repo" || projects[1].Type != "untrusted" ||
		projects[1].CanonicalName != "review.example.com/org/repo" {
		t.Errorf("projects = %+v", projects)
	}
}

func TestGetTenantStatus_PassesPayloadThrough(t *testing.T) {
	// Status values (epoch milliseconds, uuids, refs) must survive unchanged.
	const body = `{"zuul_version":"12.0.0","pipelines":[{"name":"gate","change_queues":[{"heads":[[{"id":"9f0e","enqueue_time":1795068126000,"refs":[{"id":"1,1"}]}]]}]}]}`
	c, _ := fixtureServer(t, "/api/tenant/t/status", body)
	status, err := c.GetTenantStatus(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if string(status) != body {
		t.Errorf("status = %s, want it passed through", status)
	}
}

func TestGetConfigErrors(t *testing.T) {
	c, seen := fixtureServer(t, "/api/tenant/t/config-errors", `[
		{"source_context":{"project":"org/repo","branch":"main","path":"zuul.yaml"},
		 "error":"Job lint not defined","short_error":"Job lint not defined","severity":"error","name":"Job Not Defined"}
	]`)
	configErrors, err := c.GetConfigErrors(context.Background(), "t", &ConfigErrorsQuery{
		Project: "org/repo", Branch: "main", Severity: "error", Name: "Job Not Defined", Limit: 10, Skip: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"project": {"org/repo"}, "branch": {"main"}, "severity": {"error"},
		"name": {"Job Not Defined"}, "limit": {"10"}, "skip": {"5"},
	}
	if got := seen.Query().Encode(); got != want.Encode() {
		t.Errorf("query = %q, want %q", got, want.Encode())
	}
	if len(configErrors) != 1 {
		t.Fatalf("config errors = %+v", configErrors)
	}
	e := configErrors[0]
	if e.Severity != "error" || e.Name != "Job Not Defined" || e.SourceContext == nil || e.SourceContext.Path != "zuul.yaml" {
		t.Errorf("config error = %+v", e)
	}
	// Severity and name must reach the model output too.
	out, _ := json.Marshal(e)
	for _, want := range []string{`"severity":"error"`, `"name":"Job Not Defined"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("marshalled error lost %s: %s", want, out)
		}
	}
}

func TestListEndpoints_ServerError(t *testing.T) {
	c := errorServer(t)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"ListBuildsets":   func() error { _, err := c.ListBuildsets(ctx, "t", nil); return err },
		"ListJobs":        func() error { _, err := c.ListJobs(ctx, "t"); return err },
		"ListPipelines":   func() error { _, err := c.ListPipelines(ctx, "t"); return err },
		"ListProjects":    func() error { _, err := c.ListProjects(ctx, "t"); return err },
		"GetTenantStatus": func() error { _, err := c.GetTenantStatus(ctx, "t"); return err },
		"GetConfigErrors": func() error { _, err := c.GetConfigErrors(ctx, "t", nil); return err },
	} {
		err := call()
		if err == nil || !strings.Contains(err.Error(), "500") {
			t.Errorf("%s error = %v, want a 500 error", name, err)
		}
	}
}
