// Package client provides a Zuul REST API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/clappingmonkey/zuul-mcp/internal/config"
	"github.com/clappingmonkey/zuul-mcp/internal/models"
)

// Client is a Zuul REST API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	// logClient fetches build logs from log storage. It never carries the
	// Zuul token, so it may follow redirects to other hosts.
	logClient *http.Client
	authToken string
}

// New creates a new Zuul API client.
func New(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(cfg.ZuulURL, "/"),
		httpClient: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: checkRedirect(true),
		},
		logClient: &http.Client{
			Timeout:       60 * time.Second,
			CheckRedirect: checkRedirect(false),
		},
		authToken: cfg.AuthToken,
	}
}

const maxRedirects = 5

// checkRedirect refuses long redirect chains and downgrades from https to
// http. With sameOrigin, it also refuses to leave the original scheme, host
// and port, so the Zuul token is never forwarded anywhere else.
func checkRedirect(sameOrigin bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		first := via[0].URL
		for _, hop := range via {
			if hop.URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect from https to %s://%s", req.URL.Scheme, req.URL.Host)
			}
		}
		if sameOrigin && (req.URL.Scheme != first.Scheme ||
			!strings.EqualFold(req.URL.Hostname(), first.Hostname()) ||
			effectivePort(req.URL) != effectivePort(first)) {
			return fmt.Errorf("refusing redirect to another origin (%s://%s)", req.URL.Scheme, req.URL.Host)
		}
		return nil
	}
}

// effectivePort returns u's port, filling in the scheme's default.
func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return strconv.Itoa(n)
		}
		return p
	}
	switch u.Scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}

// NewWithHTTPClient creates a new Zuul API client with a custom HTTP client,
// used for both API and log requests.
func NewWithHTTPClient(cfg *config.Config, httpClient *http.Client) *Client {
	c := New(cfg)
	c.httpClient = httpClient
	c.logClient = httpClient
	return c
}

// doRequest performs an HTTP request with optional authentication.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	// Arguments are path-escaped, but "." and ".." survive escaping and would
	// let a hostile tool argument walk to another tenant or endpoint.
	for _, seg := range strings.Split(strings.SplitN(path, "?", 2)[0], "/") {
		if seg == "." || seg == ".." {
			return nil, errors.New("invalid request: '.' and '..' are not allowed as names")
		}
	}
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
		return statusError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

// maxErrorBody bounds how much of a response is read for acknowledgement
// checks; maxErrorSnippet bounds how much of an error body is echoed back.
const (
	maxErrorBody    = 2048
	maxErrorSnippet = 512
)

// readSnippet reads a bounded prefix of an error body and makes it safe to
// show to the model: invalid UTF-8 and control characters are removed and
// truncation is marked.
func readSnippet(resp *http.Response) string {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorSnippet+1))
	truncated := len(data) > maxErrorSnippet
	if truncated {
		data = data[:maxErrorSnippet]
	}
	s := strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '
		case unicode.IsControl(r) || r == utf8.RuneError:
			return -1
		}
		return r
	}, strings.ToValidUTF8(string(data), ""))
	s = strings.TrimSpace(s)
	if truncated {
		s += "... (truncated)"
	}
	return s
}

func statusError(resp *http.Response) error {
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, readSnippet(resp))
}

// send performs a write request with an optional JSON body. zuul-web
// acknowledges successful POSTs with 200 and a bare JSON `true`, and deletes
// with 204 and no body.
// Anything else (for example an HTML login page from an auth proxy) is
// treated as a failure so a write is never reported as done when it was not.
func (c *Client) send(ctx context.Context, method, path string, payload any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case method == http.MethodDelete && resp.StatusCode == http.StatusNoContent:
		return nil
	case method != http.MethodDelete && resp.StatusCode == http.StatusOK:
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody+1))
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
		if len(data) > maxErrorBody {
			return fmt.Errorf("unexpected response from Zuul (expected `true`): %s... (truncated)", data[:maxErrorBody])
		}
		if strings.TrimSpace(string(data)) != "true" {
			return fmt.Errorf("unexpected response from Zuul (expected `true`): %s", data)
		}
		return nil
	case resp.StatusCode == http.StatusNotFound && method == http.MethodPost:
		// zuul-web only registers its POST admin routes when authentication is
		// configured, so a 404 can mean either a missing route or a missing
		// tenant/project/pipeline.
		return fmt.Errorf("not found (404): the tenant, project or pipeline does not exist, or zuul-web has no authentication configured so admin endpoints are disabled: %s", readSnippet(resp))
	default:
		return statusError(resp)
	}
}

// projectPath escapes a project name for use in a URL path. zuul-web matches
// project names with a greedy `{project_name:.*}` route and does not unquote
// them, so slashes must stay literal while each segment is escaped.
func projectPath(project string) string {
	segments := strings.Split(project, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// jobPathReplacer double-encodes the characters that would otherwise be
// altered by the second decoding stage (see jobPath).
var jobPathReplacer = strings.NewReplacer("%25", "%2525", "+", "%252B")

// jobPath escapes a job name for use in a URL path. Job names are decoded
// twice: cheroot percent-decodes the path (except %2F), then zuul-web applies
// unquote_plus. Slashes are therefore sent as %2F, while a literal '%' or '+'
// must be double-encoded so the second stage does not turn them into an
// escape sequence or a space. For example "a@b/c" becomes "a@b%2Fc" and
// "my+job" becomes "my%252Bjob".
func jobPath(job string) string {
	return jobPathReplacer.Replace(url.PathEscape(job))
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

// BuildLog is the (possibly truncated) tail of a build's job-output.txt.
type BuildLog struct {
	URL       string
	Content   string
	Truncated bool
}

// maxLogDownload caps how much is streamed from a log server that ignores
// Range requests before giving up.
const maxLogDownload = 100 << 20

// GetBuildLogs returns at most the last maxBytes of a build's job output.
// It first fetches the build to get the log URL, then requests only the tail
// of job-output.txt with a Range request, falling back to streaming the
// file while keeping only its tail.
//
// When the log is cut, output starts at the next line boundary. If the kept
// tail holds no complete line, the partial line is returned as is.
//
// The Zuul API token is never sent to the log server: logs live on separate
// storage (Swift, S3, a web server) that does not use Zuul's JWTs, and
// sending it would leak the credential to whatever host log_url points at.
func (c *Client) GetBuildLogs(ctx context.Context, tenant, uuid string, maxBytes int64) (*BuildLog, error) {
	if maxBytes <= 0 || maxBytes > maxLogDownload {
		return nil, fmt.Errorf("maxBytes must be between 1 and %d", maxLogDownload)
	}

	build, err := c.GetBuild(ctx, tenant, uuid)
	if err != nil {
		return nil, err
	}
	if build.LogURL == "" {
		return nil, fmt.Errorf("build %s has no log URL", uuid)
	}
	base, err := url.Parse(build.LogURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil {
		return nil, fmt.Errorf("build %s has an unsupported log URL", uuid)
	}
	base.Fragment = ""
	logURL := base.JoinPath("job-output.txt").String()

	// One extra byte tells whether the kept tail starts on a line boundary.
	want := maxBytes + 1
	tail, atStart, err := c.fetchLogTail(ctx, logURL, want, true)
	if errors.Is(err, errRangeNotSatisfiable) {
		// Retry without Range; readTail still keeps only the last want bytes.
		tail, atStart, err = c.fetchLogTail(ctx, logURL, want, false)
	}
	if errors.Is(err, errLogNotFound) {
		return nil, fmt.Errorf("logs not available for build %s", uuid)
	}
	if err != nil {
		return nil, err
	}

	content, truncated := trimLogTail(tail, atStart, maxBytes)
	return &BuildLog{URL: logURL, Content: content, Truncated: truncated}, nil
}

var (
	errRangeNotSatisfiable = errors.New("range not satisfiable")
	errLogNotFound         = errors.New("log not found")
)

// fetchLogTail returns at most the last want bytes of logURL and reports
// whether they start at offset 0. With useRange it asks the server for only
// those bytes; either way no more than about 2*want bytes are held in memory.
func (c *Client) fetchLogTail(ctx context.Context, logURL string, want int64, useRange bool) ([]byte, bool, error) {
	if want <= 0 {
		return nil, false, errors.New("fetching logs: want must be positive")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("creating log request: %w", err)
	}
	if useRange {
		req.Header.Set("Range", fmt.Sprintf("bytes=-%d", want))
	}
	// Ask for the raw bytes so the Range applies to what is stored.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := c.logClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetching logs: %w", err)
	}
	defer resp.Body.Close()

	limit := int64(maxLogDownload)
	switch resp.StatusCode {
	case http.StatusOK:
		buf, total, err := readTail(resp.Body, want, limit)
		if err != nil {
			return nil, false, err
		}
		return buf, total == int64(len(buf)), nil
	case http.StatusPartialContent:
		if !useRange {
			return nil, false, errors.New("fetching logs: unexpected partial response")
		}
		start, end, size, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil || (size >= 0 && end != size-1) {
			return nil, false, errors.New("fetching logs: log server returned an invalid Content-Range")
		}
		buf, total, err := readTail(resp.Body, want, limit)
		if err != nil {
			return nil, false, err
		}
		if total != end-start+1 {
			return nil, false, errors.New("fetching logs: log server sent a body that does not match its Content-Range")
		}
		return buf, start+total-int64(len(buf)) == 0, nil
	case http.StatusRequestedRangeNotSatisfiable:
		if resp.Header.Get("Content-Range") == "bytes */0" {
			return nil, true, nil
		}
		if !useRange {
			return nil, false, errors.New("fetching logs: log server returned status 416")
		}
		return nil, false, errRangeNotSatisfiable
	case http.StatusNotFound:
		return nil, false, errLogNotFound
	default:
		// Log storage is not Zuul; do not relay its error page.
		return nil, false, fmt.Errorf("fetching logs: log server returned status %d", resp.StatusCode)
	}
}

// parseContentRange parses "bytes first-last/size" ("*" size returns -1).
func parseContentRange(s string) (start, end, size int64, err error) {
	bad := fmt.Errorf("invalid Content-Range %q", s)
	rest, ok := strings.CutPrefix(s, "bytes ")
	if !ok {
		return 0, 0, 0, bad
	}
	rng, sz, ok := strings.Cut(rest, "/")
	if !ok {
		return 0, 0, 0, bad
	}
	first, last, ok := strings.Cut(rng, "-")
	if !ok {
		return 0, 0, 0, bad
	}
	if start, err = strconv.ParseInt(first, 10, 64); err != nil || start < 0 {
		return 0, 0, 0, bad
	}
	if end, err = strconv.ParseInt(last, 10, 64); err != nil || end < start {
		return 0, 0, 0, bad
	}
	size = -1
	if sz != "*" {
		if size, err = strconv.ParseInt(sz, 10, 64); err != nil || size <= end {
			return 0, 0, 0, bad
		}
	}
	return start, end, size, nil
}

// readTail streams r and returns its last keep bytes (everything when keep
// is 0) and the total number of bytes read. It fails once more than limit
// bytes have been read, without reading further.
func readTail(r io.Reader, keep, limit int64) ([]byte, int64, error) {
	var buf []byte
	var total int64
	chunk := make([]byte, 32<<10)
	lr := io.LimitReader(r, limit+1)
	for {
		n, err := lr.Read(chunk)
		if n > 0 {
			total += int64(n)
			if total > limit {
				return nil, total, fmt.Errorf("log is larger than %d bytes and the log server does not support range requests", limit)
			}
			buf = append(buf, chunk[:n]...)
			if keep > 0 && int64(len(buf)) > 2*keep {
				buf = append(buf[:0], buf[int64(len(buf))-keep:]...)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, total, fmt.Errorf("reading logs: %w", err)
		}
	}
	if keep > 0 && int64(len(buf)) > keep {
		buf = buf[int64(len(buf))-keep:]
	}
	return buf, total, nil
}

// trimLogTail turns the fetched tail (at most maxBytes+1 bytes) into at
// most maxBytes bytes of valid UTF-8, starting at a line boundary when the
// log was cut.
func trimLogTail(tail []byte, atStart bool, maxBytes int64) (string, bool) {
	if atStart && int64(len(tail)) <= maxBytes {
		return strings.ToValidUTF8(string(tail), "?"), false
	}

	content := tail
	lineStart := false
	if int64(len(tail)) > maxBytes {
		content = tail[int64(len(tail))-maxBytes:]
		lineStart = tail[int64(len(tail))-maxBytes-1] == '\n'
	}
	if !lineStart {
		if i := bytes.IndexByte(content, '\n'); i >= 0 && i < len(content)-1 {
			content = content[i+1:]
			lineStart = true
		}
	}
	if !lineStart {
		// Mid-line cut with no complete line kept: skip a split character.
		for n := 0; n < utf8.UTFMax && len(content) > 0 && !utf8.RuneStart(content[0]); n++ {
			content = content[1:]
		}
	}
	// "?" is one byte, so repairing invalid UTF-8 never grows the content.
	return strings.ToValidUTF8(string(content), "?"), true
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

// GetJob returns every variant of a job. zuul-web responds with a list of
// full variant definitions whose schema is large and undocumented, so the
// entries are passed through untyped.
func (c *Client) GetJob(ctx context.Context, tenant, jobName string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/job/%s", url.PathEscape(tenant), jobPath(jobName))

	var variants []json.RawMessage
	if err := c.get(ctx, path, &variants); err != nil {
		return nil, fmt.Errorf("getting job: %w", err)
	}
	return variants, nil
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
	path := fmt.Sprintf("/api/tenant/%s/project/%s", url.PathEscape(tenant), projectPath(projectName))

	var project models.Project
	if err := c.get(ctx, path, &project); err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return &project, nil
}

// GetTenantStatus returns the status of a tenant including pipeline queues.
// zuul-web documents this payload as "subject to change without notice"
// (e.g. queue items carry a `refs` list and millisecond integer timestamps),
// so it is passed through untyped.
func (c *Client) GetTenantStatus(ctx context.Context, tenant string) (json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/status", url.PathEscape(tenant))

	var status json.RawMessage
	if err := c.get(ctx, path, &status); err != nil {
		return nil, fmt.Errorf("getting tenant status: %w", err)
	}
	return status, nil
}

// ErrPipelineNotFound is returned by GetPipelineStatus when the tenant status
// does not contain the requested pipeline.
var ErrPipelineNotFound = errors.New("pipeline not found")

// GetPipelineStatus returns the status entry of a single pipeline from the
// tenant status, untyped for the same reason as GetTenantStatus.
func (c *Client) GetPipelineStatus(ctx context.Context, tenant, pipeline string) (json.RawMessage, error) {
	path := fmt.Sprintf("/api/tenant/%s/status", url.PathEscape(tenant))

	var status struct {
		Pipelines []json.RawMessage `json:"pipelines"`
	}
	if err := c.get(ctx, path, &status); err != nil {
		return nil, fmt.Errorf("getting tenant status: %w", err)
	}

	for _, raw := range status.Pipelines {
		var p struct {
			Name string `json:"name"`
		}
		// The payload is unstable: skip entries of an unexpected shape
		// instead of failing the lookup of a different pipeline.
		if json.Unmarshal(raw, &p) == nil && p.Name == pipeline {
			return raw, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrPipelineNotFound, pipeline)
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

// ConfigErrorsQuery holds optional filters for listing configuration errors.
// zuul-web pages this endpoint (50 entries by default).
type ConfigErrorsQuery struct {
	Project  string
	Branch   string
	Severity string
	Name     string
	Limit    int
	Skip     int
}

// GetConfigErrors returns configuration errors for a tenant.
func (c *Client) GetConfigErrors(ctx context.Context, tenant string, query *ConfigErrorsQuery) ([]models.ConfigError, error) {
	path := fmt.Sprintf("/api/tenant/%s/config-errors", url.PathEscape(tenant))

	if query != nil {
		params := url.Values{}
		for key, value := range map[string]string{
			"project":  query.Project,
			"branch":   query.Branch,
			"severity": query.Severity,
			"name":     query.Name,
		} {
			if value != "" {
				params.Set(key, value)
			}
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

	var configErrors []models.ConfigError
	if err := c.get(ctx, path, &configErrors); err != nil {
		return nil, fmt.Errorf("getting config errors: %w", err)
	}
	return configErrors, nil
}

// SystemEventsQuery holds optional query parameters for listing system events.
type SystemEventsQuery struct {
	EventType string
	Limit     int
	Skip      int
}

// ListSystemEvents returns the system event log for a tenant.
func (c *Client) ListSystemEvents(ctx context.Context, tenant string, query *SystemEventsQuery) ([]models.SystemEvent, error) {
	path := fmt.Sprintf("/api/tenant/%s/system-events", url.PathEscape(tenant))

	if query != nil {
		params := url.Values{}
		if query.EventType != "" {
			params.Set("event_type", query.EventType)
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

	var events []models.SystemEvent
	if err := c.get(ctx, path, &events); err != nil {
		return nil, fmt.Errorf("listing system events: %w", err)
	}
	return events, nil
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

// GetAutohold returns a single autohold request by its string ID.
func (c *Client) GetAutohold(ctx context.Context, tenant, requestID string) (*models.Autohold, error) {
	path := fmt.Sprintf("/api/tenant/%s/autohold/%s", url.PathEscape(tenant), url.PathEscape(requestID))

	var autohold models.Autohold
	if err := c.get(ctx, path, &autohold); err != nil {
		return nil, fmt.Errorf("getting autohold: %w", err)
	}
	return &autohold, nil
}

// CreateAutohold creates a new autohold request (requires auth). zuul-web
// does not return the created request; use ListAutoholds to find its ID.
func (c *Client) CreateAutohold(ctx context.Context, tenant, project string, req *models.AutoholdRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/autohold",
		url.PathEscape(tenant), projectPath(project))

	if err := c.send(ctx, http.MethodPost, path, req); err != nil {
		return fmt.Errorf("creating autohold: %w", err)
	}
	return nil
}

// DeleteAutohold deletes an autohold request by its string ID (requires auth).
func (c *Client) DeleteAutohold(ctx context.Context, tenant, requestID string) error {
	path := fmt.Sprintf("/api/tenant/%s/autohold/%s", url.PathEscape(tenant), url.PathEscape(requestID))

	if err := c.send(ctx, http.MethodDelete, path, nil); err != nil {
		return fmt.Errorf("deleting autohold: %w", err)
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

// ListComponents returns all registered Zuul components grouped by kind.
func (c *Client) ListComponents(ctx context.Context) (models.Components, error) {
	var components models.Components
	if err := c.get(ctx, "/api/components", &components); err != nil {
		return nil, fmt.Errorf("listing components: %w", err)
	}
	return components, nil
}

// EnqueueRequest is the body of POST /project/{project}/enqueue. Set either
// Change, or Ref with optional OldRev/NewRev.
type EnqueueRequest struct {
	Pipeline string `json:"pipeline"`
	Change   string `json:"change,omitempty"`
	Ref      string `json:"ref,omitempty"`
	OldRev   string `json:"oldrev,omitempty"`
	NewRev   string `json:"newrev,omitempty"`
}

// Enqueue enqueues a change or ref into a pipeline (requires auth).
func (c *Client) Enqueue(ctx context.Context, tenant, project string, req *EnqueueRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/enqueue",
		url.PathEscape(tenant), projectPath(project))

	if err := c.send(ctx, http.MethodPost, path, req); err != nil {
		return fmt.Errorf("enqueueing: %w", err)
	}
	return nil
}

// DequeueRequest is the body of POST /project/{project}/dequeue. Exactly one
// of Change or Ref must be set.
type DequeueRequest struct {
	Pipeline string `json:"pipeline"`
	Change   string `json:"change,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

// Dequeue dequeues a change or ref from a pipeline (requires auth).
func (c *Client) Dequeue(ctx context.Context, tenant, project string, req *DequeueRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/project/%s/dequeue",
		url.PathEscape(tenant), projectPath(project))

	if err := c.send(ctx, http.MethodPost, path, req); err != nil {
		return fmt.Errorf("dequeueing: %w", err)
	}
	return nil
}

// PromoteRequest represents a request to promote changes in a pipeline.
type PromoteRequest struct {
	Pipeline string   `json:"pipeline"`
	Changes  []string `json:"changes"`
}

// Promote moves changes to the top of their pipeline queue (requires auth).
func (c *Client) Promote(ctx context.Context, tenant string, req *PromoteRequest) error {
	path := fmt.Sprintf("/api/tenant/%s/promote", url.PathEscape(tenant))

	if err := c.send(ctx, http.MethodPost, path, req); err != nil {
		return fmt.Errorf("promoting changes: %w", err)
	}
	return nil
}
