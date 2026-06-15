// Package client provides a Zuul REST API client.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/models"
)

// Client is a Zuul REST API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// New creates a new Zuul API client.
func New(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(cfg.ZuulURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authToken: cfg.AuthToken,
	}
}

// NewWithHTTPClient creates a new Zuul API client with a custom HTTP client.
func NewWithHTTPClient(cfg *config.Config, httpClient *http.Client) *Client {
	c := New(cfg)
	c.httpClient = httpClient
	return c
}

// doRequest performs an HTTP request with optional authentication.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// get performs a GET request and decodes the JSON response.
func (c *Client) get(ctx context.Context, path string, result any) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// ListTenants returns all tenants.
func (c *Client) ListTenants(ctx context.Context) ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := c.get(ctx, "/api/tenants", &tenants); err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	return tenants, nil
}

// BuildsQuery holds optional query parameters for listing builds.
type BuildsQuery struct {
	Project  string
	Pipeline string
	Change   int
	Branch   string
	Ref      string
	Result   string
	UUID     string
	JobName  string
	Voting   *bool
	Limit    int
	Skip     int
}

// ListBuilds returns builds for a tenant with optional filters.
func (c *Client) ListBuilds(ctx context.Context, tenant string, query *BuildsQuery) ([]models.Build, error) {
	path := fmt.Sprintf("/api/tenant/%s/builds", url.PathEscape(tenant))

	if query != nil {
		params := url.Values{}
		if query.Project != "" {
			params.Set("project", query.Project)
		}
		if query.Pipeline != "" {
			params.Set("pipeline", query.Pipeline)
		}
		if query.Change > 0 {
			params.Set("change", strconv.Itoa(query.Change))
		}
		if query.Branch != "" {
			params.Set("branch", query.Branch)
		}
		if query.Ref != "" {
			params.Set("ref", query.Ref)
		}
		if query.Result != "" {
			params.Set("result", query.Result)
		}
		if query.UUID != "" {
			params.Set("uuid", query.UUID)
		}
		if query.JobName != "" {
			params.Set("job_name", query.JobName)
		}
		if query.Voting != nil {
			params.Set("voting", strconv.FormatBool(*query.Voting))
		}
		if query.Limit > 0 {
			params.Set("limit", strconv.Itoa(query.Limit))
		}
		if query.Skip > 0 {
			params.Set("skip", strconv.Itoa(query.Skip))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	var builds []models.Build
	if err := c.get(ctx, path, &builds); err != nil {
		return nil, fmt.Errorf("listing builds: %w", err)
	}
	return builds, nil
}

// GetBuild returns a specific build by UUID.
func (c *Client) GetBuild(ctx context.Context, tenant, uuid string) (*models.Build, error) {
	path := fmt.Sprintf("/api/tenant/%s/build/%s", url.PathEscape(tenant), url.PathEscape(uuid))

	var build models.Build
	if err := c.get(ctx, path, &build); err != nil {
		return nil, fmt.Errorf("getting build: %w", err)
	}
	return &build, nil
}

// GetBuildLogs returns the job output logs for a specific build.
// It first fetches the build to get the log URL, then fetches the logs.
func (c *Client) GetBuildLogs(ctx context.Context, tenant, uuid string) (string, error) {
	// Get build to find log_url
	build, err := c.GetBuild(ctx, tenant, uuid)
	if err != nil {
		return "", err
	}

	if build.LogURL == "" {
		return "", fmt.Errorf("build %s has no log URL", uuid)
	}

	// Fetch logs from log_url/job-output.txt
	logURL := strings.TrimSuffix(build.LogURL, "/") + "/job-output.txt"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating log request: %w", err)
	}

	// Log storage might not need auth, but include it if available
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("logs not available for build %s", uuid)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching logs", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading logs: %w", err)
	}

	return string(body), nil
}

// BuildsetsQuery holds optional query parameters for listing buildsets.
type BuildsetsQuery struct {
	Project  string
	Pipeline string
	Change   int
	Branch   string
	Ref      string
	Result   string
	Limit    int
	Skip     int
}

// ListBuildsets returns buildsets for a tenant with optional filters.
func (c *Client) ListBuildsets(ctx context.Context, tenant string, query *BuildsetsQuery) ([]models.Buildset, error) {
	path := fmt.Sprintf("/api/tenant/%s/buildsets", url.PathEscape(tenant))

	if query != nil {
		params := url.Values{}
		if query.Project != "" {
			params.Set("project", query.Project)
		}
		if query.Pipeline != "" {
			params.Set("pipeline", query.Pipeline)
		}
		if query.Change > 0 {
			params.Set("change", strconv.Itoa(query.Change))
		}
		if query.Branch != "" {
			params.Set("branch", query.Branch)
		}
		if query.Ref != "" {
			params.Set("ref", query.Ref)
		}
		if query.Result != "" {
			params.Set("result", query.Result)
		}
		if query.Limit > 0 {
			params.Set("limit", strconv.Itoa(query.Limit))
		}
		if query.Skip > 0 {
			params.Set("skip", strconv.Itoa(query.Skip))
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	var buildsets []models.Buildset
	if err := c.get(ctx, path, &buildsets); err != nil {
		return nil, fmt.Errorf("listing buildsets: %w", err)
	}
	return buildsets, nil
}

// GetBuildset returns a specific buildset by UUID.
func (c *Client) GetBuildset(ctx context.Context, tenant, uuid string) (*models.Buildset, error) {
	path := fmt.Sprintf("/api/tenant/%s/buildset/%s", url.PathEscape(tenant), url.PathEscape(uuid))

	var buildset models.Buildset
	if err := c.get(ctx, path, &buildset); err != nil {
		return nil, fmt.Errorf("getting buildset: %w", err)
	}
	return &buildset, nil
}

// ListJobs returns all jobs for a tenant.
func (c *Client) ListJobs(ctx context.Context, tenant string) ([]models.Job, error) {
	path := fmt.Sprintf("/api/tenant/%s/jobs", url.PathEscape(tenant))

	var jobs []models.Job
	if err := c.get(ctx, path, &jobs); err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	return jobs, nil
}

// GetJob returns details of a specific job.
func (c *Client) GetJob(ctx context.Context, tenant, jobName string) (*models.Job, error) {
	path := fmt.Sprintf("/api/tenant/%s/job/%s", url.PathEscape(tenant), url.PathEscape(jobName))

	var job models.Job
	if err := c.get(ctx, path, &job); err != nil {
		return nil, fmt.Errorf("getting job: %w", err)
	}
	return &job, nil
}

// ListPipelines returns all pipelines for a tenant.
func (c *Client) ListPipelines(ctx context.Context, tenant string) ([]models.Pipeline, error) {
	path := fmt.Sprintf("/api/tenant/%s/pipelines", url.PathEscape(tenant))

	var pipelines []models.Pipeline
	if err := c.get(ctx, path, &pipelines); err != nil {
		return nil, fmt.Errorf("listing pipelines: %w", err)
	}
	return pipelines, nil
}

// ListProjects returns all projects for a tenant.
func (c *Client) ListProjects(ctx context.Context, tenant string) ([]models.Project, error) {
	path := fmt.Sprintf("/api/tenant/%s/projects", url.PathEscape(tenant))

	var projects []models.Project
	if err := c.get(ctx, path, &projects); err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	return projects, nil
}

// GetProject returns details of a specific project.
func (c *Client) GetProject(ctx context.Context, tenant, projectName string) (*models.Project, error) {
	path := fmt.Sprintf("/api/tenant/%s/project/%s", url.PathEscape(tenant), url.PathEscape(projectName))

	var project models.Project
	if err := c.get(ctx, path, &project); err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return &project, nil
}

// GetTenantStatus returns the status of a tenant including pipeline queues.
func (c *Client) GetTenantStatus(ctx context.Context, tenant string) (*models.TenantStatus, error) {
	path := fmt.Sprintf("/api/tenant/%s/status", url.PathEscape(tenant))

	var status models.TenantStatus
	if err := c.get(ctx, path, &status); err != nil {
		return nil, fmt.Errorf("getting tenant status: %w", err)
	}
	return &status, nil
}

// GetChangeStatus returns the pipeline queue status for a specific change.
// The response is a slice of raw pipeline status objects, one per pipeline
// where the change is currently queued.
//
// NOTE: The Zuul REST API documentation explicitly states that the output
// format for this endpoint is "not currently documented and subject to change
// without notice". The raw response is returned as-is to remain forward
// compatible with undocumented field additions or removals.
func (c *Client) GetChangeStatus(ctx context.Context, tenant, change string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/status/change/%s",
		url.PathEscape(tenant), url.PathEscape(change))

	var pipelines []json.RawMessage
	if err := c.get(ctx, path, &pipelines); err != nil {
		return nil, fmt.Errorf("getting change status: %w", err)
	}
	return pipelines, nil
}

// GetConfigErrors returns configuration errors for a tenant.
func (c *Client) GetConfigErrors(ctx context.Context, tenant string) ([]models.ConfigError, error) {
	path := fmt.Sprintf("/api/tenant/%s/config-errors", url.PathEscape(tenant))

	var errors []models.ConfigError
	if err := c.get(ctx, path, &errors); err != nil {
		return nil, fmt.Errorf("getting config errors: %w", err)
	}
	return errors, nil
}

// ListAutoholds returns all autohold requests for a tenant.
func (c *Client) ListAutoholds(ctx context.Context, tenant string) ([]models.Autohold, error) {
	path := fmt.Sprintf("/api/tenant/%s/autohold", url.PathEscape(tenant))

	var autoholds []models.Autohold
	if err := c.get(ctx, path, &autoholds); err != nil {
		return nil, fmt.Errorf("listing autoholds: %w", err)
	}
	return autoholds, nil
}

// GetAutohold returns a single autohold request by ID.
// The response includes the Nodes field listing held node IDs, which is absent
// in the list endpoint response.
func (c *Client) GetAutohold(ctx context.Context, tenant string, requestID int) (*models.Autohold, error) {
	path := fmt.Sprintf("/api/tenant/%s/autohold/%d", url.PathEscape(tenant), requestID)

	var autohold models.Autohold
	if err := c.get(ctx, path, &autohold); err != nil {
		return nil, fmt.Errorf("getting autohold: %w", err)
	}
	return &autohold, nil
}

// CreateAutohold creates a new autohold request.
func (c *Client) CreateAutohold(ctx context.Context, tenant, project, job string, req *models.AutoholdRequest) (*models.Autohold, error) {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/autohold/%s",
		url.PathEscape(tenant), url.PathEscape(project), url.PathEscape(job))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creating autohold: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var autohold models.Autohold
	if err := json.NewDecoder(resp.Body).Decode(&autohold); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &autohold, nil
}

// DeleteAutohold deletes an autohold request.
func (c *Client) DeleteAutohold(ctx context.Context, tenant string, requestID int) error {
	path := fmt.Sprintf("/api/tenant/%s/autohold/%d", url.PathEscape(tenant), requestID)

	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting autohold: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListNodes returns the list of nodepool nodes for a tenant.
// TODO: Replace json.RawMessage with a typed Node model once the Zuul API
// response schema for /nodes is documented and verified against a live instance.
func (c *Client) ListNodes(ctx context.Context, tenant string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/nodes", url.PathEscape(tenant))

	var nodes []json.RawMessage
	if err := c.get(ctx, path, &nodes); err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	return nodes, nil
}

// ListLabels returns the list of available node labels for a tenant.
// TODO: Replace json.RawMessage with a typed Label model once the Zuul API
// response schema for /labels is documented and verified against a live instance.
func (c *Client) ListLabels(ctx context.Context, tenant string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/labels", url.PathEscape(tenant))

	var labels []json.RawMessage
	if err := c.get(ctx, path, &labels); err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}
	return labels, nil
}

// ListConnections returns the list of Zuul connections.
// TODO: Replace json.RawMessage with the typed Connection model once the Zuul API
// response schema for /connections is documented and verified against a live instance.
func (c *Client) ListConnections(ctx context.Context) ([]json.RawMessage, error) {
	var connections []json.RawMessage
	if err := c.get(ctx, "/api/connections", &connections); err != nil {
		return nil, fmt.Errorf("listing connections: %w", err)
	}
	return connections, nil
}

// ListSemaphores returns the list of semaphores for a tenant.
// TODO: Replace json.RawMessage with the typed Semaphore model once the Zuul API
// response schema for /semaphores is documented and verified against a live instance.
func (c *Client) ListSemaphores(ctx context.Context, tenant string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/semaphores", url.PathEscape(tenant))

	var semaphores []json.RawMessage
	if err := c.get(ctx, path, &semaphores); err != nil {
		return nil, fmt.Errorf("listing semaphores: %w", err)
	}
	return semaphores, nil
}

// ListComponents returns all Zuul components (schedulers, executors, mergers, etc.).
// TODO: Replace with verified schema once the Zuul API response for /api/components
// is formally documented (currently not in the OpenAPI spec).
func (c *Client) ListComponents(ctx context.Context) (*models.Components, error) {
	var components models.Components
	if err := c.get(ctx, "/api/components", &components); err != nil {
		return nil, fmt.Errorf("listing components: %w", err)
	}
	return &components, nil
}

// GetJobVariants returns the list of variants for a specific job.
// TODO: Replace json.RawMessage with a typed JobVariant model once the Zuul API
// response schema for /job/{name}/variants is documented and verified against a live instance.
func (c *Client) GetJobVariants(ctx context.Context, tenant, jobName string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/job/%s/variants", url.PathEscape(tenant), url.PathEscape(jobName))

	var variants []json.RawMessage
	if err := c.get(ctx, path, &variants); err != nil {
		return nil, fmt.Errorf("getting job variants: %w", err)
	}
	return variants, nil
}

// EnqueueRequest represents a request to enqueue a change.
type EnqueueRequest struct {
	Pipeline string `json:"pipeline"`
	Change   string `json:"change"`
	Trigger  string `json:"trigger,omitempty"`
}

// Enqueue enqueues a change into a pipeline (requires auth).
func (c *Client) Enqueue(ctx context.Context, tenant, project string, req *EnqueueRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/enqueue",
		url.PathEscape(tenant), url.PathEscape(project))

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("enqueueing change: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// DequeueRequest represents a request to dequeue a change.
type DequeueRequest struct {
	Pipeline string `json:"pipeline"`
	Change   string `json:"change,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

// Dequeue dequeues a change or ref from a pipeline (requires auth).
func (c *Client) Dequeue(ctx context.Context, tenant, project string, req *DequeueRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/dequeue",
		url.PathEscape(tenant), url.PathEscape(project))

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("dequeueing change: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// PromoteRequest represents a request to promote changes in a pipeline.
type PromoteRequest struct {
	Pipeline string   `json:"pipeline"`
	Changes  []string `json:"changes"`
}

// Promote reorders changes in a pipeline by moving them to the top of the queue (requires auth).
func (c *Client) Promote(ctx context.Context, tenant string, req *PromoteRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/promote", url.PathEscape(tenant))

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("promoting changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
