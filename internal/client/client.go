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
