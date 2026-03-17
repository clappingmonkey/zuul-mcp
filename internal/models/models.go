// Package models defines data structures for Zuul API responses.
package models

import "time"

// Tenant represents a Zuul tenant.
type Tenant struct {
	Name     string   `json:"name"`
	Projects []string `json:"projects,omitempty"`
}

// Build represents a Zuul build.
type Build struct {
	UUID         string     `json:"uuid"`
	JobName      string     `json:"job_name"`
	Result       string     `json:"result,omitempty"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Duration     float64    `json:"duration,omitempty"`
	Voting       bool       `json:"voting"`
	LogURL       string     `json:"log_url,omitempty"`
	NodeName     string     `json:"node_name,omitempty"`
	Project      string     `json:"project"`
	Branch       string     `json:"branch,omitempty"`
	Pipeline     string     `json:"pipeline"`
	Change       int        `json:"change,omitempty"`
	Patchset     string     `json:"patchset,omitempty"`
	Ref          string     `json:"ref,omitempty"`
	RefURL       string     `json:"ref_url,omitempty"`
	EventID      string     `json:"event_id,omitempty"`
	BuildsetUUID string     `json:"buildset_uuid,omitempty"`
}

// Buildset represents a Zuul buildset (a collection of builds for a change).
type Buildset struct {
	UUID       string     `json:"uuid"`
	Result     string     `json:"result,omitempty"`
	Message    string     `json:"message,omitempty"`
	Project    string     `json:"project"`
	Branch     string     `json:"branch,omitempty"`
	Pipeline   string     `json:"pipeline"`
	Change     int        `json:"change,omitempty"`
	Patchset   string     `json:"patchset,omitempty"`
	Ref        string     `json:"ref,omitempty"`
	RefURL     string     `json:"ref_url,omitempty"`
	EventID    string     `json:"event_id,omitempty"`
	FirstBuild *time.Time `json:"first_build_start_time,omitempty"`
	LastBuild  *time.Time `json:"last_build_end_time,omitempty"`
	Builds     []Build    `json:"builds,omitempty"`
}

// Job represents a Zuul job definition.
type Job struct {
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Parent        string         `json:"parent,omitempty"`
	Branches      []string       `json:"branches,omitempty"`
	Vars          any            `json:"vars,omitempty"`
	Nodeset       any            `json:"nodeset,omitempty"`
	Timeout       int            `json:"timeout,omitempty"`
	Voting        bool           `json:"voting"`
	Abstract      bool           `json:"abstract"`
	Protected     bool           `json:"protected"`
	Final         bool           `json:"final"`
	SourceContext *SourceContext `json:"source_context,omitempty"`
}

// SourceContext represents where a Zuul configuration was defined.
type SourceContext struct {
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Path    string `json:"path"`
}

// Pipeline represents a Zuul pipeline.
type Pipeline struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Manager          string `json:"manager,omitempty"`
	Precedence       string `json:"precedence,omitempty"`
	TriggerEventType string `json:"trigger_event_type,omitempty"`
}

// PipelineStatus represents the current status of a pipeline.
type PipelineStatus struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ChangeQueues []ChangeQueue `json:"change_queues,omitempty"`
}

// ChangeQueue represents a queue of changes in a pipeline.
type ChangeQueue struct {
	Name  string        `json:"name"`
	Heads [][]QueueItem `json:"heads,omitempty"`
}

// QueueItem represents an item in a change queue.
type QueueItem struct {
	ID            string      `json:"id"`
	Project       string      `json:"project"`
	Branch        string      `json:"branch,omitempty"`
	Change        int         `json:"change,omitempty"`
	Patchset      string      `json:"patchset,omitempty"`
	Ref           string      `json:"ref,omitempty"`
	EnqueueTime   *time.Time  `json:"enqueue_time,omitempty"`
	RemainingTime int         `json:"remaining_time,omitempty"`
	Jobs          []JobStatus `json:"jobs,omitempty"`
}

// JobStatus represents the status of a job in a queue.
type JobStatus struct {
	Name          string  `json:"name"`
	URL           string  `json:"url,omitempty"`
	Result        string  `json:"result,omitempty"`
	Voting        bool    `json:"voting"`
	StartTime     *int64  `json:"start_time,omitempty"`
	ElapsedTime   *int64  `json:"elapsed_time,omitempty"`
	RemainingTime *int64  `json:"remaining_time,omitempty"`
	Worker        *Worker `json:"worker,omitempty"`
}

// Worker represents a worker executing a job.
type Worker struct {
	Name     string `json:"name,omitempty"`
	Hostname string `json:"hostname,omitempty"`
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

// Autohold represents an autohold request.
type Autohold struct {
	ID             int        `json:"id"`
	Tenant         string     `json:"tenant"`
	Project        string     `json:"project"`
	Job            string     `json:"job"`
	RefFilter      string     `json:"ref_filter"`
	MaxCount       int        `json:"max_count"`
	CurrentCount   int        `json:"current_count"`
	Reason         string     `json:"reason"`
	NodeExpiration int        `json:"node_expiration,omitempty"`
	RequestTime    *time.Time `json:"request_time,omitempty"`
	RequestedBy    string     `json:"requested_by,omitempty"`
}

// AutoholdRequest represents a request to create an autohold.
type AutoholdRequest struct {
	ChangeFilter   string `json:"change,omitempty"`
	RefFilter      string `json:"ref_filter,omitempty"`
	Reason         string `json:"reason"`
	Count          int    `json:"count"`
	NodeExpiration int    `json:"node_expiration,omitempty"`
}

// ConfigError represents a Zuul configuration error.
type ConfigError struct {
	SourceContext *SourceContext `json:"source_context,omitempty"`
	Error         string         `json:"error"`
	ShortError    string         `json:"short_error,omitempty"`
}

// TenantStatus represents the overall status of a tenant.
type TenantStatus struct {
	Pipelines        []PipelineStatus `json:"pipelines,omitempty"`
	ZuulVersion      string           `json:"zuul_version,omitempty"`
	LastReconfigured *time.Time       `json:"last_reconfigured,omitempty"`
}

// Connection represents a Zuul connection.
type Connection struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// Semaphore represents a Zuul semaphore.
type Semaphore struct {
	Name   string `json:"name"`
	Global bool   `json:"global"`
	Max    int    `json:"max"`
}
