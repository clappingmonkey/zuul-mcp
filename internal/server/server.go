// Package server provides the MCP server implementation for Zuul.
package server

import (
	"fmt"
	"math"
	"strings"

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

// New creates a new Zuul MCP server. version is reported to clients in the
// initialize response.
func New(cfg *config.Config, version string) *Server {
	mcpServer := server.NewMCPServer(
		"zuul-mcp",
		version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithInstructions(instructions(cfg)),
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

// WriteToolsEnabled reports whether the tools that change Zuul state are
// offered: they need ZUUL_AUTH_TOKEN and are hidden in read-only mode.
func WriteToolsEnabled(cfg *config.Config) bool {
	return WriteToolsDisabledReason(cfg) == ""
}

// WriteToolsDisabledReason explains why the write tools are hidden, or
// returns "" when they are offered.
func WriteToolsDisabledReason(cfg *config.Config) string {
	switch {
	case cfg.ReadOnly:
		return "read-only mode is enabled (ZUUL_READ_ONLY or -read-only)"
	case !cfg.HasAuth():
		return "ZUUL_AUTH_TOKEN is not set"
	default:
		return ""
	}
}

func instructions(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("Tools for the Zuul CI system: builds, buildsets, logs, jobs, pipelines, projects, nodes and autoholds.\n")
	if cfg.DefaultTenant != "" {
		fmt.Fprintf(&b, "The tenant argument is optional and defaults to %q; call list_tenants to see others.\n", cfg.DefaultTenant)
	} else {
		b.WriteString("Tenant-scoped tools need a tenant argument; call list_tenants first if you do not know it.\n")
	}
	b.WriteString("get_change_status, enqueue, dequeue and promote take Gerrit changes as '<number>,<patchset>' (e.g. '12345,1') and GitHub pull requests as their number; the change filter of list_builds and list_buildsets is the number only.\n")
	b.WriteString("To investigate a failure: list_builds (filter by project, change or result), then get_build and get_build_logs with tail_lines.\n")
	if WriteToolsEnabled(cfg) {
		b.WriteString("enqueue, dequeue, promote, create_autohold and delete_autohold change Zuul state; confirm with the user before calling them.\n")
	} else {
		fmt.Fprintf(&b, "This server is read-only because %s: tools that change Zuul state (enqueue, dequeue, promote, autoholds) are not available.\n", WriteToolsDisabledReason(cfg))
	}
	return b.String()
}

// Annotation presets. Every tool talks to a Zuul server (open world).
var (
	readTool = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	additiveWriteTool = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	destructiveWriteTool = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(true),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
)

type toolMeta struct {
	title       string
	annotations mcp.ToolAnnotation
}

// toolCatalog holds the title and behaviour hints for every tool. addTool
// panics for tools missing here so new tools cannot ship unannotated.
var toolCatalog = map[string]toolMeta{
	"list_tenants":        {"List tenants", readTool},
	"list_builds":         {"List builds", readTool},
	"get_build":           {"Get build", readTool},
	"get_build_logs":      {"Get build log", readTool},
	"list_buildsets":      {"List buildsets", readTool},
	"get_buildset":        {"Get buildset", readTool},
	"list_jobs":           {"List jobs", readTool},
	"get_job":             {"Get job", readTool},
	"list_pipelines":      {"List pipelines", readTool},
	"get_pipeline_status": {"Get pipeline status", readTool},
	"list_projects":       {"List projects", readTool},
	"get_project":         {"Get project", readTool},
	"get_tenant_status":   {"Get tenant status", readTool},
	"get_change_status":   {"Get change status", readTool},
	"get_config_errors":   {"Get config errors", readTool},
	"list_system_events":  {"List system events", readTool},
	"list_nodes":          {"List nodes", readTool},
	"list_labels":         {"List labels", readTool},
	"list_connections":    {"List connections", readTool},
	"list_components":     {"List components", readTool},
	"list_semaphores":     {"List semaphores", readTool},
	"list_autoholds":      {"List autoholds", readTool},
	"get_autohold":        {"Get autohold", readTool},
	"create_autohold":     {"Create autohold", additiveWriteTool},
	"enqueue":             {"Enqueue change or ref", additiveWriteTool},
	"delete_autohold":     {"Delete autohold", destructiveWriteTool},
	"dequeue":             {"Dequeue change or ref", destructiveWriteTool},
	"promote":             {"Promote changes", destructiveWriteTool},
}

// addTool attaches the catalog annotations and registers the tool. Tools
// that change Zuul state are skipped unless WriteToolsEnabled.
func (s *Server) addTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	meta, ok := toolCatalog[tool.Name]
	if !ok {
		panic(fmt.Sprintf("tool %q has no entry in toolCatalog", tool.Name))
	}
	readOnly := meta.annotations.ReadOnlyHint != nil && *meta.annotations.ReadOnlyHint
	if !readOnly && !WriteToolsEnabled(s.config) {
		return
	}
	tool.Annotations = meta.annotations
	tool.Annotations.Title = meta.title
	s.mcpServer.AddTool(tool, handler)
}

// registerTools registers all Zuul MCP tools.
func (s *Server) registerTools() {
	// List tenants
	s.addTool(
		mcp.NewTool("list_tenants",
			mcp.WithDescription("List all Zuul tenants"),
		),
		s.handleListTenants,
	)

	// List builds
	s.addTool(
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
			mcp.WithInteger("change",
				mcp.Description("Filter by change number (e.g. 12345, without patchset)"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of results to return (default 50, max 1000)"),
				mcp.DefaultNumber(defaultListLimit),
				mcp.Min(1),
				mcp.Max(maxListLimit),
			),
			mcp.WithInteger("skip",
				mcp.Description("Number of results to skip for pagination"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
		),
		s.handleListBuilds,
	)

	// Get build
	s.addTool(
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
	s.addTool(
		mcp.NewTool("get_build_logs",
			mcp.WithDescription("Get the end of the job output log (job-output.txt) for a specific build. Large logs are truncated to the last max_bytes bytes; the full log URL is included when output is truncated."),
			mcp.WithString("uuid",
				mcp.Required(),
				mcp.Description("Build UUID"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithInteger("tail_lines",
				mcp.Description("Return only the last N lines of the fetched log"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
			mcp.WithInteger("max_bytes",
				mcp.Description("Maximum number of bytes to fetch from the end of the log (default 100000, max 1000000)"),
				mcp.DefaultNumber(defaultLogBytes),
				mcp.Min(1),
				mcp.Max(maxLogBytes),
			),
		),
		s.handleGetBuildLogs,
	)

	// List buildsets
	s.addTool(
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
			mcp.WithInteger("change",
				mcp.Description("Filter by change number (e.g. 12345, without patchset)"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of results to return (default 50, max 1000)"),
				mcp.DefaultNumber(defaultListLimit),
				mcp.Min(1),
				mcp.Max(maxListLimit),
			),
			mcp.WithInteger("skip",
				mcp.Description("Number of results to skip for pagination"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
		),
		s.handleListBuildsets,
	)

	// Get buildset
	s.addTool(
		mcp.NewTool("get_buildset",
			mcp.WithDescription("Get details of a specific buildset by UUID"),
			mcp.WithString("uuid",
				mcp.Required(),
				mcp.Description("Buildset UUID"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetBuildset,
	)

	// List jobs
	s.addTool(
		mcp.NewTool("list_jobs",
			mcp.WithDescription("List all jobs defined in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListJobs,
	)

	// Get job
	s.addTool(
		mcp.NewTool("get_job",
			mcp.WithDescription("Get the full definition of a job. Returns a list with one entry per job variant (branch- or project-specific definitions)."),
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
	s.addTool(
		mcp.NewTool("list_pipelines",
			mcp.WithDescription("List all pipelines in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListPipelines,
	)

	// Get pipeline status
	s.addTool(
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
	s.addTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List all projects in a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListProjects,
	)

	// Get project
	s.addTool(
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
	s.addTool(
		mcp.NewTool("get_tenant_status",
			mcp.WithDescription("Get overall status of a tenant including all pipeline queues"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetTenantStatus,
	)

	// Get change status
	s.addTool(
		mcp.NewTool("get_change_status",
			mcp.WithDescription("Get pipeline status for a specific change. Returns the raw pipeline queue entries for the change across all pipelines where it is currently queued. Note: the Zuul API marks this endpoint's output format as unstable."),
			mcp.WithString("change",
				mcp.Required(),
				mcp.Description("Change ID (e.g. '12345,1' for Gerrit, '42' for GitHub PR)"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetChangeStatus,
	)

	// Get config errors
	s.addTool(
		mcp.NewTool("get_config_errors",
			mcp.WithDescription("Get configuration errors for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetConfigErrors,
	)

	// List system events
	s.addTool(
		mcp.NewTool("list_system_events",
			mcp.WithDescription("List system events for a tenant (e.g. config updates, reconfigurations)"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
			mcp.WithString("event_type",
				mcp.Description("Filter by event type (e.g. 'config_update', 'reconfiguration')"),
			),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of results to return (max 1000; Zuul default if omitted)"),
				mcp.Min(1),
				mcp.Max(maxListLimit),
			),
			mcp.WithInteger("skip",
				mcp.Description("Number of results to skip for pagination"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
		),
		s.handleListSystemEvents,
	)

	// List nodes
	s.addTool(
		mcp.NewTool("list_nodes",
			mcp.WithDescription("List nodepool nodes for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListNodes,
	)

	// List labels
	s.addTool(
		mcp.NewTool("list_labels",
			mcp.WithDescription("List available node labels for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListLabels,
	)

	// List connections
	s.addTool(
		mcp.NewTool("list_connections",
			mcp.WithDescription("List all Zuul connections"),
		),
		s.handleListConnections,
	)

	// List components
	s.addTool(
		mcp.NewTool("list_components",
			mcp.WithDescription("List all Zuul components (schedulers, executors, mergers, etc.)"),
		),
		s.handleListComponents,
	)

	// List semaphores
	s.addTool(
		mcp.NewTool("list_semaphores",
			mcp.WithDescription("List semaphores for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListSemaphores,
	)

	// List autoholds
	s.addTool(
		mcp.NewTool("list_autoholds",
			mcp.WithDescription("List all autohold requests for a tenant"),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleListAutoholds,
	)

	// Get autohold
	s.addTool(
		mcp.NewTool("get_autohold",
			mcp.WithDescription("Get details of a specific autohold request, including held nodes"),
			mcp.WithString("request_id",
				mcp.Required(),
				mcp.Description("Autohold request ID as returned by list_autoholds (e.g. '0000000001')"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleGetAutohold,
	)

	// Create autohold
	s.addTool(
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
			mcp.WithString("change",
				mcp.Description("Only hold builds for this change (e.g. '12345'). Mutually exclusive with 'ref'"),
			),
			mcp.WithString("ref",
				mcp.Description("Only hold builds whose ref matches this regular expression. Mutually exclusive with 'change'"),
			),
			mcp.WithInteger("count",
				mcp.Description("Number of failed builds to hold nodes for"),
				mcp.DefaultNumber(1),
				mcp.Min(1),
				mcp.Max(math.MaxInt32),
			),
			mcp.WithInteger("node_hold_expiration",
				mcp.Description("Seconds to keep held nodes before they are released. Omit to use the Zuul default; 0 means no expiration (capped at the configured maximum, if any)"),
				mcp.Min(0),
				mcp.Max(math.MaxInt32),
			),
		),
		s.handleCreateAutohold,
	)

	// Delete autohold
	s.addTool(
		mcp.NewTool("delete_autohold",
			mcp.WithDescription("Delete an autohold request (requires authentication)"),
			mcp.WithString("request_id",
				mcp.Required(),
				mcp.Description("Autohold request ID as returned by list_autoholds (e.g. '0000000001')"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleDeleteAutohold,
	)

	// Enqueue change
	s.addTool(
		mcp.NewTool("enqueue",
			mcp.WithDescription("Enqueue a change or ref into a pipeline (requires authentication). Provide exactly one of 'change' or 'ref'."),
			mcp.WithString("project",
				mcp.Required(),
				mcp.Description("Project name"),
			),
			mcp.WithString("pipeline",
				mcp.Required(),
				mcp.Description("Pipeline name"),
			),
			mcp.WithString("change",
				mcp.Description("Change ID in format <number>,<patchset> (e.g., '12345,1')"),
			),
			mcp.WithString("ref",
				mcp.Description("Git ref to enqueue (e.g. 'refs/heads/main')"),
			),
			mcp.WithString("oldrev",
				mcp.Description("Old revision SHA for a ref update (only with 'ref')"),
			),
			mcp.WithString("newrev",
				mcp.Description("New revision SHA for a ref update (only with 'ref')"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleEnqueue,
	)

	// Dequeue change
	s.addTool(
		mcp.NewTool("dequeue",
			mcp.WithDescription("Dequeue a change or ref from a pipeline (requires authentication). Provide exactly one of 'change' or 'ref'."),
			mcp.WithString("project",
				mcp.Required(),
				mcp.Description("Project name"),
			),
			mcp.WithString("pipeline",
				mcp.Required(),
				mcp.Description("Pipeline name"),
			),
			mcp.WithString("change",
				mcp.Description("Change ID in format <number>,<patchset> (provide change or ref)"),
			),
			mcp.WithString("ref",
				mcp.Description("Git ref name (provide change or ref)"),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handleDequeue,
	)

	// Promote changes
	s.addTool(
		mcp.NewTool("promote",
			mcp.WithDescription("Promote changes to the top of a pipeline queue (requires authentication)"),
			mcp.WithString("pipeline",
				mcp.Required(),
				mcp.Description("Pipeline name"),
			),
			mcp.WithArray("changes",
				mcp.Required(),
				mcp.Description("Change IDs in format <number>,<patchset>, in the desired queue order (e.g. [\"12345,1\", \"13336,3\"])"),
				mcp.WithStringItems(),
				mcp.MinItems(1),
			),
			mcp.WithString("tenant",
				mcp.Description("Tenant name (uses default if not specified)"),
			),
		),
		s.handlePromote,
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
