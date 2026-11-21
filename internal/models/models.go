// Package models defines data structures for Zuul API responses.
package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ZuulTime is a custom time type that handles Zuul's timestamp formats.
// Zuul may return timestamps without timezone suffix (e.g., "2026-03-06T07:24:19")
// which Go's time.Time cannot parse by default.
type ZuulTime struct {
	time.Time
}

// zuulTimeFormats lists the timestamp formats to try, in order of preference.
// time.Parse interprets values without a zone offset as UTC, which matches
// zuul-web's naive "%Y-%m-%dT%H:%M:%S" output.
var zuulTimeFormats = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
}

// UnmarshalJSON implements json.Unmarshaler for ZuulTime.
func (zt *ZuulTime) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		return nil
	}

	// Remove quotes
	s := strings.Trim(string(data), "\"")
	if s == "" {
		return nil
	}

	// Try each format
	var parseErr error
	for _, format := range zuulTimeFormats {
		t, err := time.Parse(format, s)
		if err == nil {
			zt.Time = t
			return nil
		}
		parseErr = err
	}

	return fmt.Errorf("unable to parse time %q: %w", s, parseErr)
}

// MarshalJSON implements json.Marshaler for ZuulTime.
func (zt ZuulTime) MarshalJSON() ([]byte, error) {
	if zt.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", zt.Time.Format(time.RFC3339))), nil
}

// Tenant represents a Zuul tenant.
type Tenant struct {
	Name     string   `json:"name"`
	Projects []string `json:"projects,omitempty"`
}

// Ref represents a Zuul ref (change/PR reference).
// In the Zuul API, ref information is returned as a nested object in builds
// and as an array of refs in buildsets.
type Ref struct {
	Project  string `json:"project"`
	Branch   string `json:"branch,omitempty"`
	Change   int    `json:"change,omitempty"`
	Patchset string `json:"patchset,omitempty"`
	Ref      string `json:"ref,omitempty"`
	OldRev   string `json:"oldrev,omitempty"`
	NewRev   string `json:"newrev,omitempty"`
	RefURL   string `json:"ref_url,omitempty"`
}

// Build represents a Zuul build.
type Build struct {
	UUID           string    `json:"uuid"`
	JobName        string    `json:"job_name"`
	Result         string    `json:"result,omitempty"`
	Held           bool      `json:"held,omitempty"`
	ExecuteTime    *ZuulTime `json:"execute_time,omitempty"`
	StartTime      *ZuulTime `json:"start_time,omitempty"`
	EndTime        *ZuulTime `json:"end_time,omitempty"`
	Duration       float64   `json:"duration,omitempty"`
	Voting         bool      `json:"voting"`
	LogURL         string    `json:"log_url,omitempty"`
	Nodeset        string    `json:"nodeset,omitempty"`
	ErrorDetail    string    `json:"error_detail,omitempty"`
	Final          bool      `json:"final,omitempty"`
	Pipeline       string    `json:"pipeline"`
	Ref            *Ref      `json:"ref,omitempty"`
	EventID        string    `json:"event_id,omitempty"`
	EventTimestamp *ZuulTime `json:"event_timestamp,omitempty"`
	QueueItemUUID  string    `json:"queue_item_uuid,omitempty"`
}

// Buildset represents a Zuul buildset (a collection of builds for a change).
type Buildset struct {
	UUID           string    `json:"uuid"`
	Result         string    `json:"result,omitempty"`
	Message        string    `json:"message,omitempty"`
	Pipeline       string    `json:"pipeline"`
	EventID        string    `json:"event_id,omitempty"`
	EventTimestamp *ZuulTime `json:"event_timestamp,omitempty"`
	QueueItemUUID  string    `json:"queue_item_uuid,omitempty"`
	FirstBuild     *ZuulTime `json:"first_build_start_time,omitempty"`
	LastBuild      *ZuulTime `json:"last_build_end_time,omitempty"`
	Refs           []Ref     `json:"refs,omitempty"`
	Builds         []Build   `json:"builds,omitempty"`
}

// Job is an entry of GET /api/tenant/{tenant}/jobs.
// Full job definitions come from GET /job/{name}, which returns a list of
// variants and is passed through untyped.
type Job struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Variants    []JobVariant `json:"variants,omitempty"`
}

// JobVariant summarizes one variant of a job in the /jobs listing.
type JobVariant struct {
	Parent   string   `json:"parent,omitempty"`
	Branches []string `json:"branches,omitempty"`
}

// SourceContext represents where a Zuul configuration was defined.
type SourceContext struct {
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Path    string `json:"path"`
}

// Pipeline represents a Zuul pipeline.
type Pipeline struct {
	Name     string            `json:"name"`
	Triggers []PipelineTrigger `json:"triggers,omitempty"`
}

// PipelineTrigger identifies a trigger of a pipeline.
type PipelineTrigger struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// Project represents a Zuul project.
type Project struct {
	Name           string          `json:"name"`
	ConnectionName string          `json:"connection_name,omitempty"`
	CanonicalName  string          `json:"canonical_name,omitempty"`
	Type           string          `json:"type,omitempty"`
	Configs        []ProjectConfig `json:"configs,omitempty"`
}

// ProjectConfig represents project configuration for a pipeline.
type ProjectConfig struct {
	Pipeline string      `json:"pipeline"`
	Jobs     []JobConfig `json:"jobs,omitempty"`
}

// JobConfig represents job configuration in a project.
type JobConfig struct {
	Name string `json:"name"`
}

// Autohold represents an autohold request as returned by the list and get
// endpoints. IDs are zero-padded strings (e.g. "0000000001") and must be
// passed back verbatim.
type Autohold struct {
	ID             string          `json:"id"`
	Tenant         string          `json:"tenant"`
	Project        string          `json:"project"`
	Job            string          `json:"job"`
	RefFilter      string          `json:"ref_filter"`
	MaxCount       int             `json:"max_count"`
	CurrentCount   int             `json:"current_count"`
	Reason         string          `json:"reason"`
	NodeExpiration int             `json:"node_expiration"`
	Expired        json.RawMessage `json:"expired,omitempty"`
	Nodes          json.RawMessage `json:"nodes,omitempty"`
}

// AutoholdRequest is the body of POST /project/{project}/autohold.
// zuul-web rejects the request unless every key is present, so nil pointers
// are sent as JSON null rather than omitted. Change and Ref are mutually
// exclusive; a nil NodeHoldExpiration selects the tenant default.
type AutoholdRequest struct {
	Job                string  `json:"job"`
	Change             *string `json:"change"`
	Ref                *string `json:"ref"`
	Reason             string  `json:"reason"`
	Count              int     `json:"count"`
	NodeHoldExpiration *int    `json:"node_hold_expiration"`
}

// ConfigError represents a Zuul configuration error.
type ConfigError struct {
	SourceContext *SourceContext `json:"source_context,omitempty"`
	Error         string         `json:"error"`
	ShortError    string         `json:"short_error,omitempty"`
	Severity      string         `json:"severity,omitempty"`
	Name          string         `json:"name,omitempty"`
}

// ComponentInfo represents a single Zuul component instance.
type ComponentInfo struct {
	Hostname string `json:"hostname"`
	State    string `json:"state"`
	Version  string `json:"version"`
}

// Components is the response body for GET /api/components, keyed by
// component kind ("scheduler", "executor", "merger", "fingergw", "web",
// "launcher"). A kind is present only if at least one instance is registered.
type Components map[string][]ComponentInfo

// SystemEvent represents a single entry from GET /api/tenant/{tenant}/system-events.
// This is one of the few Zuul API endpoints with a documented response schema.
type SystemEvent struct {
	Description string    `json:"description"`
	EventID     string    `json:"event_id"`
	EventTime   *ZuulTime `json:"event_time,omitempty"`
	EventType   string    `json:"event_type"`
}
