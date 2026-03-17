// Package server provides the MCP server implementation for Zuul.
package server

import (
	"github.com/clappingmonkey/zuul-mcp/internal/client"
	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with Zuul-specific functionality.
type Server struct {
	mcpServer     *server.MCPServer
	zuulClient    *client.Client
	config        *config.Config
	defaultTenant string
}

// New creates a new Zuul MCP server.
func New(cfg *config.Config) *Server {
	mcpServer := server.NewMCPServer(
		"zuul-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	s := &Server{
		mcpServer:     mcpServer,
		zuulClient:    client.New(cfg),
		config:        cfg,
		defaultTenant: cfg.DefaultTenant,
	}

	s.registerTools()

	return s
}

// MCPServer returns the underlying MCP server.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// registerTools registers all Zuul MCP tools.
func (s *Server) registerTools() {
	// List tenants
	s.mcpServer.AddTool(
		mcp.NewTool("list_tenants",
			mcp.WithDescription("List all Zuul tenants"),
		),
		s.handleListTenants,
	)

	// List builds
	s.mcpServer.AddTool(
		mcp.NewTool("list_builds",
			mcp.WithDescription("List builds for a tenant with optional filters"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithString("project",
				mcp.Description("Filter by project name"),
			),
			mcp.WithString("pipeline",
				mcp.Description("Filter by pipeline name"),
			),
			mcp.WithString("branch",
				mcp.Description("Filter by branch name"),
			),
			mcp.WithString("result",
				mcp.Description("Filter by result (SUCCESS, FAILURE, etc.)"),
			),
			mcp.WithString("job_name",
				mcp.Description("Filter by job name"),
			),
			mcp.WithNumber("change",
				mcp.Description("Filter by change number"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default: 50)"),
			),
			mcp.WithNumber("skip",
				mcp.Description("Number of results to skip for pagination"),
			),
		),
		s.handleListBuilds,
	)

	// Get build
	s.mcpServer.AddTool(
		mcp.NewTool("get_build",
			mcp.WithDescription("Get details of a specific build by UUID"),
			mcp.WithString("uuid",
				mcp.Required(),
				mcp.Description("Build UUID"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetBuild,
	)

	// Get build logs
	s.mcpServer.AddTool(
		mcp.NewTool("get_build_logs",
			mcp.WithDescription("Get the job output logs for a specific build"),
			mcp.WithString("uuid",
				mcp.Required(),
				mcp.Description("Build UUID"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithNumber("tail_lines",
				mcp.Description("Return only the last N lines of the log (optional, returns full log if not specified)"),
			),
		),
		s.handleGetBuildLogs,
	)

	// List buildsets
	s.mcpServer.AddTool(
		mcp.NewTool("list_buildsets",
			mcp.WithDescription("List buildsets for a tenant with optional filters"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithString("project",
				mcp.Description("Filter by project name"),
			),
			mcp.WithString("pipeline",
				mcp.Description("Filter by pipeline name"),
			),
			mcp.WithString("branch",
				mcp.Description("Filter by branch name"),
			),
			mcp.WithString("result",
				mcp.Description("Filter by result (SUCCESS, FAILURE, etc.)"),
			),
			mcp.WithNumber("change",
				mcp.Description("Filter by change number"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default: 50)"),
			),
			mcp.WithNumber("skip",
				mcp.Description("Number of results to skip for pagination"),
			),
		),
		s.handleListBuildsets,
	)

	// List jobs
	s.mcpServer.AddTool(
		mcp.NewTool("list_jobs",
			mcp.WithDescription("List all jobs defined in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListJobs,
	)

	// Get job
	s.mcpServer.AddTool(
		mcp.NewTool("get_job",
			mcp.WithDescription("Get details of a specific job"),
			mcp.WithString("job_name",
				mcp.Required(),
				mcp.Description("Job name"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetJob,
	)

	// List pipelines
	s.mcpServer.AddTool(
		mcp.NewTool("list_pipelines",
			mcp.WithDescription("List all pipelines in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListPipelines,
	)

	// Get pipeline status
	s.mcpServer.AddTool(
		mcp.NewTool("get_pipeline_status",
			mcp.WithDescription("Get current status of a specific pipeline including queue"),
			mcp.WithString("pipeline",
				mcp.Required(),
				mcp.Description("Pipeline name"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetPipelineStatus,
	)

	// List projects
	s.mcpServer.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List all projects in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListProjects,
	)

	// Get project
	s.mcpServer.AddTool(
		mcp.NewTool("get_project",
			mcp.WithDescription("Get details of a specific project"),
			mcp.WithString("project",
				mcp.Required(),
				mcp.Description("Project name"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetProject,
	)

	// Get tenant status
	s.mcpServer.AddTool(
		mcp.NewTool("get_tenant_status",
			mcp.WithDescription("Get overall status of a tenant including all pipeline queues"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetTenantStatus,
	)

	// Get config errors
	s.mcpServer.AddTool(
		mcp.NewTool("get_config_errors",
			mcp.WithDescription("Get configuration errors for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetConfigErrors,
	)

	// List autoholds
	s.mcpServer.AddTool(
		mcp.NewTool("list_autoholds",
			mcp.WithDescription("List all autohold requests for a tenant (requires authentication)"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListAutoholds,
	)

	// Create autohold
	s.mcpServer.AddTool(
		mcp.NewTool("create_autohold",
			mcp.WithDescription("Create an autohold request to hold nodes after job failure (requires authentication)"),
			mcp.WithString("project",
				mcp.Required(),
				mcp.Description("Project name"),
			),
			mcp.WithString("job",
				mcp.Required(),
				mcp.Description("Job name"),
			),
			mcp.WithString("reason",
				mcp.Required(),
				mcp.Description("Reason for the autohold request"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithString("ref_filter",
				mcp.Description("Regular expression to match refs"),
			),
			mcp.WithString("change",
				mcp.Description("Change filter"),
			),
			mcp.WithNumber("count",
				mcp.Description("Number of times to hold (default: 1)"),
			),
			mcp.WithNumber("node_expiration",
				mcp.Description("Node hold expiration in seconds"),
			),
		),
		s.handleCreateAutohold,
	)

	// Delete autohold
	s.mcpServer.AddTool(
		mcp.NewTool("delete_autohold",
			mcp.WithDescription("Delete an autohold request (requires authentication)"),
			mcp.WithNumber("request_id",
				mcp.Required(),
				mcp.Description("Autohold request ID"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleDeleteAutohold,
	)
}

// getTenant returns the tenant from request or falls back to default.
func (s *Server) getTenant(req mcp.CallToolRequest) (string, error) {
	tenant := req.GetString("tenant", "")
	if tenant != "" {
		return tenant, nil
	}
	if s.defaultTenant != "" {
		return s.defaultTenant, nil
	}
	return "", ErrNoTenant
}
