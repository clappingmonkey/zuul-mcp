package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clappingmonkey/zuul-mcp/internal/client"
	"github.com/clappingmonkey/zuul-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// ErrNoTenant is returned when no tenant is specified and no default is configured.
var ErrNoTenant = errors.New("tenant is required: specify 'tenant' parameter or set ZUUL_DEFAULT_TENANT environment variable")

// ErrNoAuth is returned when an authenticated operation is attempted without auth.
var ErrNoAuth = errors.New("authentication required: set ZUUL_AUTH_TOKEN environment variable")

// jsonResult converts a value to a JSON string for tool results.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// handleListTenants handles the list_tenants tool.
func (s *Server) handleListTenants(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenants, err := s.zuulClient.ListTenants(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list tenants: %v", err)), nil
	}
	return jsonResult(tenants)
}

// handleListBuilds handles the list_builds tool.
func (s *Server) handleListBuilds(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query := &client.BuildsQuery{
		Project:  req.GetString("project", ""),
		Pipeline: req.GetString("pipeline", ""),
		Branch:   req.GetString("branch", ""),
		Result:   req.GetString("result", ""),
		JobName:  req.GetString("job_name", ""),
		Change:   int(req.GetFloat("change", 0)),
		Limit:    int(req.GetFloat("limit", 50)),
		Skip:     int(req.GetFloat("skip", 0)),
	}

	builds, err := s.zuulClient.ListBuilds(ctx, tenant, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list builds: %v", err)), nil
	}
	return jsonResult(builds)
}

// handleGetBuild handles the get_build tool.
func (s *Server) handleGetBuild(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	uuid, err := req.RequireString("uuid")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	build, err := s.zuulClient.GetBuild(ctx, tenant, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get build: %v", err)), nil
	}
	return jsonResult(build)
}

// handleListBuildsets handles the list_buildsets tool.
func (s *Server) handleListBuildsets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query := &client.BuildsetsQuery{
		Project:  req.GetString("project", ""),
		Pipeline: req.GetString("pipeline", ""),
		Branch:   req.GetString("branch", ""),
		Result:   req.GetString("result", ""),
		Change:   int(req.GetFloat("change", 0)),
		Limit:    int(req.GetFloat("limit", 50)),
		Skip:     int(req.GetFloat("skip", 0)),
	}

	buildsets, err := s.zuulClient.ListBuildsets(ctx, tenant, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list buildsets: %v", err)), nil
	}
	return jsonResult(buildsets)
}

// handleListJobs handles the list_jobs tool.
func (s *Server) handleListJobs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jobs, err := s.zuulClient.ListJobs(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list jobs: %v", err)), nil
	}
	return jsonResult(jobs)
}

// handleGetJob handles the get_job tool.
func (s *Server) handleGetJob(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jobName, err := req.RequireString("job_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	job, err := s.zuulClient.GetJob(ctx, tenant, jobName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get job: %v", err)), nil
	}
	return jsonResult(job)
}

// handleListPipelines handles the list_pipelines tool.
func (s *Server) handleListPipelines(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipelines, err := s.zuulClient.ListPipelines(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list pipelines: %v", err)), nil
	}
	return jsonResult(pipelines)
}

// handleGetPipelineStatus handles the get_pipeline_status tool.
func (s *Server) handleGetPipelineStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipelineName, err := req.RequireString("pipeline")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get tenant status which contains all pipeline statuses
	status, err := s.zuulClient.GetTenantStatus(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get tenant status: %v", err)), nil
	}

	// Find the specific pipeline
	for _, p := range status.Pipelines {
		if p.Name == pipelineName {
			return jsonResult(p)
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("pipeline '%s' not found", pipelineName)), nil
}

// handleListProjects handles the list_projects tool.
func (s *Server) handleListProjects(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	projects, err := s.zuulClient.ListProjects(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
	}
	return jsonResult(projects)
}

// handleGetProject handles the get_project tool.
func (s *Server) handleGetProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	projectName, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	project, err := s.zuulClient.GetProject(ctx, tenant, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get project: %v", err)), nil
	}
	return jsonResult(project)
}

// handleGetTenantStatus handles the get_tenant_status tool.
func (s *Server) handleGetTenantStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	status, err := s.zuulClient.GetTenantStatus(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get tenant status: %v", err)), nil
	}
	return jsonResult(status)
}

// handleGetConfigErrors handles the get_config_errors tool.
func (s *Server) handleGetConfigErrors(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	errors, err := s.zuulClient.GetConfigErrors(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get config errors: %v", err)), nil
	}
	return jsonResult(errors)
}

// handleListAutoholds handles the list_autoholds tool.
func (s *Server) handleListAutoholds(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	autoholds, err := s.zuulClient.ListAutoholds(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list autoholds: %v", err)), nil
	}
	return jsonResult(autoholds)
}

// handleCreateAutohold handles the create_autohold tool.
func (s *Server) handleCreateAutohold(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.HasAuth() {
		return mcp.NewToolResultError(ErrNoAuth.Error()), nil
	}

	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	project, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	job, err := req.RequireString("job")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	reason, err := req.RequireString("reason")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Build the autohold request
	ahReq := &models.AutoholdRequest{
		Reason:         reason,
		RefFilter:      req.GetString("ref_filter", ""),
		ChangeFilter:   req.GetString("change", ""),
		Count:          int(req.GetFloat("count", 1)),
		NodeExpiration: int(req.GetFloat("node_expiration", 0)),
	}

	autohold, err := s.zuulClient.CreateAutohold(ctx, tenant, project, job, ahReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create autohold: %v", err)), nil
	}
	return jsonResult(autohold)
}

// handleDeleteAutohold handles the delete_autohold tool.
func (s *Server) handleDeleteAutohold(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.HasAuth() {
		return mcp.NewToolResultError(ErrNoAuth.Error()), nil
	}

	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	requestID, err := req.RequireFloat("request_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.zuulClient.DeleteAutohold(ctx, tenant, int(requestID))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete autohold: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted autohold request %d", int(requestID))), nil
}
