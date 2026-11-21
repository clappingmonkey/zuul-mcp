package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/clappingmonkey/zuul-mcp/internal/client"
	"github.com/clappingmonkey/zuul-mcp/internal/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// ErrNoTenant is returned when no tenant is specified and no default is configured.
var ErrNoTenant = errors.New("tenant is required: specify 'tenant' parameter or set ZUUL_DEFAULT_TENANT environment variable")

// ErrNoAuth is returned when an authenticated operation is attempted without auth.
var ErrNoAuth = errors.New("authentication required: set ZUUL_AUTH_TOKEN environment variable")

// jsonResult converts a value to compact JSON for tool results (indentation
// only costs tokens). A nil list is rendered as [] rather than null.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() && rv.Type() != reflect.TypeFor[json.RawMessage]() {
		v = []any{}
	}
	data, err := json.Marshal(v)
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

	page, err := listArgs(req.GetArguments(), defaultListLimit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	f, err := optFilters(req.GetArguments(), "project", "pipeline", "branch", "result", "job_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	query := &client.BuildsQuery{
		Project:  f["project"],
		Pipeline: f["pipeline"],
		Branch:   f["branch"],
		Result:   f["result"],
		JobName:  f["job_name"],
		Change:   page.change,
		Limit:    page.limit,
		Skip:     page.skip,
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

	uuid, err := requireString(req, "uuid")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	build, err := s.zuulClient.GetBuild(ctx, tenant, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get build: %v", err)), nil
	}
	return jsonResult(build)
}

// handleGetBuildset handles the get_buildset tool.
func (s *Server) handleGetBuildset(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	uuid, err := requireString(req, "uuid")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	buildset, err := s.zuulClient.GetBuildset(ctx, tenant, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get buildset: %v", err)), nil
	}
	return jsonResult(buildset)
}

// handleListBuildsets handles the list_buildsets tool.
func (s *Server) handleListBuildsets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	page, err := listArgs(req.GetArguments(), defaultListLimit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	f, err := optFilters(req.GetArguments(), "project", "pipeline", "branch", "result")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	query := &client.BuildsetsQuery{
		Project:  f["project"],
		Pipeline: f["pipeline"],
		Branch:   f["branch"],
		Result:   f["result"],
		Change:   page.change,
		Limit:    page.limit,
		Skip:     page.skip,
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

	jobName, err := requireString(req, "job_name")
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

	pipelineName, err := requireString(req, "pipeline")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	status, err := s.zuulClient.GetPipelineStatus(ctx, tenant, pipelineName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pipeline status: %v", err)), nil
	}
	return jsonResult(status)
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

	projectName, err := requireString(req, "project")
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

// handleGetChangeStatus handles the get_change_status tool.
// The Zuul API marks this endpoint's output format as "not currently documented
// and subject to change without notice", so the raw response is returned as-is.
func (s *Server) handleGetChangeStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	change, err := requireString(req, "change")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipelines, err := s.zuulClient.GetChangeStatus(ctx, tenant, change)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get change status: %v", err)), nil
	}
	return jsonResult(pipelines)
}

// handleGetConfigErrors handles the get_config_errors tool.
func (s *Server) handleGetConfigErrors(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := req.GetArguments()
	page, err := listArgs(args, defaultListLimit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	filters, err := optFilters(args, "project", "branch", "severity", "name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if sev := filters["severity"]; sev != "" && sev != "error" && sev != "warning" {
		return mcp.NewToolResultError("'severity' must be 'error' or 'warning'"), nil
	}
	query := &client.ConfigErrorsQuery{
		Project:  filters["project"],
		Branch:   filters["branch"],
		Severity: filters["severity"],
		Name:     filters["name"],
		Limit:    page.limit,
		Skip:     page.skip,
	}

	configErrors, err := s.zuulClient.GetConfigErrors(ctx, tenant, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get config errors: %v", err)), nil
	}
	return jsonResult(configErrors)
}

// handleListSystemEvents handles the list_system_events tool.
func (s *Server) handleListSystemEvents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// An omitted limit (returned as 0) leaves the page size to Zuul; an
	// explicit limit must still be within 1..maxListLimit.
	page, err := listArgs(req.GetArguments(), 0)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	eventType, err := optFilter(req.GetArguments(), "event_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	query := &client.SystemEventsQuery{
		EventType: eventType,
		Limit:     page.limit,
		Skip:      page.skip,
	}

	events, err := s.zuulClient.ListSystemEvents(ctx, tenant, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list system events: %v", err)), nil
	}
	return jsonResult(events)
}

// handleListNodes handles the list_nodes tool.
// TODO: Return typed Node objects once the Zuul API response schema is verified.
func (s *Server) handleListNodes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	nodes, err := s.zuulClient.ListNodes(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list nodes: %v", err)), nil
	}
	return jsonResult(nodes)
}

// handleListLabels handles the list_labels tool.
// TODO: Return typed Label objects once the Zuul API response schema is verified.
func (s *Server) handleListLabels(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	labels, err := s.zuulClient.ListLabels(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list labels: %v", err)), nil
	}
	return jsonResult(labels)
}

// handleListConnections handles the list_connections tool.
// TODO: Return typed Connection objects once the Zuul API response schema is verified.
func (s *Server) handleListConnections(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connections, err := s.zuulClient.ListConnections(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list connections: %v", err)), nil
	}
	return jsonResult(connections)
}

// handleListComponents handles the list_components tool.
func (s *Server) handleListComponents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	components, err := s.zuulClient.ListComponents(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list components: %v", err)), nil
	}
	return jsonResult(components)
}

// handleListSemaphores handles the list_semaphores tool.
// TODO: Return typed Semaphore objects once the Zuul API response schema is verified.
func (s *Server) handleListSemaphores(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	semaphores, err := s.zuulClient.ListSemaphores(ctx, tenant)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list semaphores: %v", err)), nil
	}
	return jsonResult(semaphores)
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

// handleGetAutohold handles the get_autohold tool.
func (s *Server) handleGetAutohold(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	requestID, err := autoholdID(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	autohold, err := s.zuulClient.GetAutohold(ctx, tenant, requestID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get autohold: %v", err)), nil
	}
	return jsonResult(autohold)
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

	project, err := requireString(req, "project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	job, err := requireString(req, "job")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	reason, err := requireString(req, "reason")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := req.GetArguments()

	// These parameters were advertised by earlier releases. Silently ignoring
	// them would widen the hold to every ref or drop the expiration, so fail.
	if _, ok := args["ref_filter"]; ok {
		return mcp.NewToolResultError("'ref_filter' is no longer supported: use 'ref' (a regular expression) or 'change'"), nil
	}
	if _, ok := args["node_expiration"]; ok {
		return mcp.NewToolResultError("'node_expiration' is no longer supported: use 'node_hold_expiration'"), nil
	}

	change, err := optString(args, "change")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ref, err := optString(args, "ref")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if change != "" && ref != "" {
		return mcp.NewToolResultError("'change' and 'ref' are mutually exclusive"), nil
	}

	count, err := optInt(args, "count", 1, 1, math.MaxInt32)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ahReq := &models.AutoholdRequest{
		Job:    job,
		Reason: reason,
		Count:  int(count),
	}
	if change != "" {
		ahReq.Change = &change
	}
	if ref != "" {
		ahReq.Ref = &ref
	}
	if v, ok := args["node_hold_expiration"]; ok && v != nil {
		exp, err := optInt(args, "node_hold_expiration", 0, 0, math.MaxInt32)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		e := int(exp)
		ahReq.NodeHoldExpiration = &e
	}

	if err := s.zuulClient.CreateAutohold(ctx, tenant, project, ahReq); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create autohold: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"Created autohold request for job %s in project %s (count %d). Use list_autoholds to see its ID.",
		job, project, ahReq.Count)), nil
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

	requestID, err := autoholdID(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.zuulClient.DeleteAutohold(ctx, tenant, requestID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete autohold: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted autohold request %s", requestID)), nil
}

// maxAutoholdID is the largest ZooKeeper sequence number (10 digits).
const maxAutoholdID = 9_999_999_999

// autoholdID reads the request_id argument. Zuul autohold IDs are 10-digit
// zero-padded ZooKeeper sequence numbers such as "0000000001"; shorter digit
// strings and bare numbers are padded to that form for convenience.
func autoholdID(req mcp.CallToolRequest) (string, error) {
	invalid := errors.New("'request_id' is required and must be an autohold ID such as \"0000000001\"")
	var n int64
	switch v := req.GetArguments()["request_id"].(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || len(s) > 10 || strings.Trim(s, "0123456789") != "" {
			return "", invalid
		}
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return "", invalid
		}
		n = parsed
	case float64:
		if math.IsNaN(v) || v < 0 || v > maxAutoholdID || v != math.Trunc(v) {
			return "", invalid
		}
		n = int64(v)
	default:
		return "", invalid
	}
	return fmt.Sprintf("%010d", n), nil
}

// requireString reads a required string argument and rejects blank values,
// which mcp-go does not catch on its own.
func requireString(req mcp.CallToolRequest, key string) (string, error) {
	s, err := req.RequireString(key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("'%s' must not be blank", key)
	}
	return s, nil
}

// optString reads an optional string argument. Absent or null yields "". The
// value is returned verbatim (it may be a regex), and a supplied but blank
// value is an error: silently treating it as absent would widen filters such
// as an autohold's change/ref to "match everything".
func optString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("'%s' must not be blank; omit it instead", key)
	}
	return s, nil
}

// optFilter reads an optional string filter of a read-only query. Absent,
// null or blank yields "" (no filter); any other non-string is an error so
// that a mistyped filter is not silently dropped.
func optFilter(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	return strings.TrimSpace(s), nil
}

// optFilters reads several optional string filters with optFilter.
func optFilters(args map[string]any, keys ...string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		v, err := optFilter(args, key)
		if err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}

// optInt reads an optional integer argument within [lo, hi]. Absent or null
// yields def. JSON numbers must be finite whole numbers; decimal strings are
// also accepted because some clients send numbers as strings.
func optInt(args map[string]any, key string, def, lo, hi int64) (int64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	rangeErr := fmt.Errorf("'%s' must be an integer between %d and %d", key, lo, hi)
	var n int64
	switch t := v.(type) {
	case float64:
		if math.IsNaN(t) || t < float64(lo) || t > float64(hi) || t != math.Trunc(t) {
			return 0, rangeErr
		}
		n = int64(t)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, rangeErr
		}
		n = parsed
	default:
		return 0, rangeErr
	}
	if n < lo || n > hi {
		return 0, rangeErr
	}
	return n, nil
}

const (
	defaultListLimit = 50
	maxListLimit     = 1000
)

type listPage struct {
	change, limit, skip int
}

// listArgs validates the optional change, limit and skip arguments of the
// list tools. A zero change means no change filter.
func listArgs(args map[string]any, defLimit int64) (listPage, error) {
	change, err := optInt(args, "change", 0, 0, math.MaxInt32)
	if err != nil {
		return listPage{}, err
	}
	limit, err := optInt(args, "limit", defLimit, 1, maxListLimit)
	if err != nil {
		return listPage{}, err
	}
	skip, err := optInt(args, "skip", 0, 0, math.MaxInt32)
	if err != nil {
		return listPage{}, err
	}
	return listPage{change: int(change), limit: int(limit), skip: int(skip)}, nil
}

// handleGetBuildLogs handles the get_build_logs tool.
func (s *Server) handleGetBuildLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	uuid, err := requireString(req, "uuid")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := req.GetArguments()
	tailLines, err := optInt(args, "tail_lines", 0, 0, math.MaxInt32)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	maxBytes, err := optInt(args, "max_bytes", defaultLogBytes, 1, maxLogBytes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	buildLog, err := s.zuulClient.GetBuildLogs(ctx, tenant, uuid, maxBytes)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get build logs: %v", err)), nil
	}

	content := buildLog.Content
	truncated := buildLog.Truncated
	if tailLines > 0 {
		if tail, cut := lastLines(content, tailLines); cut {
			content = tail
			truncated = true
		}
	}

	if truncated {
		content = fmt.Sprintf("[showing the end of the log only; full log: %s]\n%s", buildLog.URL, content)
	}
	return mcp.NewToolResultText(content), nil
}

// lastLines returns the last n lines of s (a trailing newline does not start
// a new line) and whether anything was dropped. It scans backwards without
// splitting, so it does not allocate per line.
func lastLines(s string, n int64) (string, bool) {
	end := len(s)
	if strings.HasSuffix(s, "\n") {
		end--
	}
	for i := end - 1; i >= 0; i-- {
		if s[i] == '\n' {
			n--
			if n == 0 {
				return s[i+1:], true
			}
		}
	}
	return s, false
}

const (
	defaultLogBytes = 100_000
	maxLogBytes     = 1_000_000
)

// handleEnqueue handles the enqueue tool.
func (s *Server) handleEnqueue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.HasAuth() {
		return mcp.NewToolResultError(ErrNoAuth.Error()), nil
	}

	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	project, err := requireString(req, "project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipeline, err := requireString(req, "pipeline")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	enqReq := &client.EnqueueRequest{Pipeline: pipeline}
	args := req.GetArguments()
	for key, dst := range map[string]*string{
		"change": &enqReq.Change,
		"ref":    &enqReq.Ref,
		"oldrev": &enqReq.OldRev,
		"newrev": &enqReq.NewRev,
	} {
		if *dst, err = optString(args, key); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	target, err := exactlyOne(enqReq.Change, enqReq.Ref)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if enqReq.Change != "" && (enqReq.OldRev != "" || enqReq.NewRev != "") {
		return mcp.NewToolResultError("'oldrev' and 'newrev' can only be used with 'ref'"), nil
	}

	err = s.zuulClient.Enqueue(ctx, tenant, project, enqReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to enqueue: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully enqueued %s into pipeline %s for project %s", target, pipeline, project)), nil
}

// exactlyOne checks that exactly one of change or ref is set and returns it.
func exactlyOne(change, ref string) (string, error) {
	switch {
	case change != "" && ref != "":
		return "", errors.New("provide either 'change' or 'ref', not both")
	case change != "":
		return change, nil
	case ref != "":
		return ref, nil
	default:
		return "", errors.New("either 'change' or 'ref' must be provided")
	}
}

// handleDequeue handles the dequeue tool.
func (s *Server) handleDequeue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.HasAuth() {
		return mcp.NewToolResultError(ErrNoAuth.Error()), nil
	}

	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	project, err := requireString(req, "project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipeline, err := requireString(req, "pipeline")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	change, err := optString(req.GetArguments(), "change")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ref, err := optString(req.GetArguments(), "ref")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	target, err := exactlyOne(change, ref)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deqReq := &client.DequeueRequest{
		Pipeline: pipeline,
		Change:   change,
		Ref:      ref,
	}

	err = s.zuulClient.Dequeue(ctx, tenant, project, deqReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to dequeue: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully dequeued %s from pipeline %s for project %s", target, pipeline, project)), nil
}

// handlePromote handles the promote tool.
func (s *Server) handlePromote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !s.config.HasAuth() {
		return mcp.NewToolResultError(ErrNoAuth.Error()), nil
	}

	tenant, err := s.getTenant(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pipeline, err := requireString(req, "pipeline")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	changes, err := changeList(req.GetArguments()["changes"])
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	promReq := &client.PromoteRequest{
		Pipeline: pipeline,
		Changes:  changes,
	}

	err = s.zuulClient.Promote(ctx, tenant, promReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to promote changes: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully promoted %d change(s) in pipeline %s", len(changes), pipeline)), nil
}

// changeList normalizes the promote "changes" argument. Change IDs contain
// commas ("12345,1"), so they cannot be comma-separated. An array is
// preferred, and a whitespace-separated string is also accepted.
func changeList(v any) ([]string, error) {
	var changes []string
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("'changes' must be an array of change ID strings")
			}
			if s = strings.TrimSpace(s); s == "" {
				return nil, errors.New("'changes' must not contain blank entries")
			}
			changes = append(changes, s)
		}
	case string:
		changes = strings.Fields(t)
	}
	if len(changes) == 0 {
		return nil, errors.New("'changes' must contain at least one change ID (e.g. [\"12345,1\", \"13336,3\"])")
	}
	return changes, nil
}
