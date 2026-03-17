package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestZuulTimeUnmarshal_RFC3339(t *testing.T) {
	input := `"2026-03-06T07:24:19Z"`
	var zt ZuulTime
	if err := json.Unmarshal([]byte(input), &zt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 6, 7, 24, 19, 0, time.UTC)
	if !zt.Time.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, zt.Time)
	}
}

func TestZuulTimeUnmarshal_RFC3339Offset(t *testing.T) {
	input := `"2026-03-06T08:24:19+01:00"`
	var zt ZuulTime
	if err := json.Unmarshal([]byte(input), &zt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// +01:00 means it's 07:24:19 UTC
	if zt.Time.UTC().Hour() != 7 {
		t.Errorf("expected hour 7 UTC, got %d", zt.Time.UTC().Hour())
	}
}

func TestZuulTimeUnmarshal_ZuulFormat(t *testing.T) {
	input := `"2026-03-06T07:24:19"`
	var zt ZuulTime
	if err := json.Unmarshal([]byte(input), &zt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be parsed as UTC
	expected := time.Date(2026, 3, 6, 7, 24, 19, 0, time.UTC)
	if !zt.Time.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, zt.Time)
	}
	// Verify it's UTC
	if zt.Time.Location() != time.UTC {
		t.Errorf("expected UTC location, got %v", zt.Time.Location())
	}
}

func TestZuulTimeUnmarshal_Null(t *testing.T) {
	input := `null`
	var zt ZuulTime
	if err := json.Unmarshal([]byte(input), &zt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !zt.IsZero() {
		t.Errorf("expected zero time, got %v", zt.Time)
	}
}

func TestZuulTimeUnmarshal_Empty(t *testing.T) {
	input := `""`
	var zt ZuulTime
	if err := json.Unmarshal([]byte(input), &zt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !zt.IsZero() {
		t.Errorf("expected zero time, got %v", zt.Time)
	}
}

func TestZuulTimeUnmarshal_Invalid(t *testing.T) {
	input := `"not-a-date"`
	var zt ZuulTime
	err := json.Unmarshal([]byte(input), &zt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestZuulTimeMarshal(t *testing.T) {
	zt := ZuulTime{Time: time.Date(2026, 3, 6, 7, 24, 19, 0, time.UTC)}
	data, err := json.Marshal(zt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `"2026-03-06T07:24:19Z"`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestZuulTimeMarshal_Zero(t *testing.T) {
	var zt ZuulTime
	data, err := json.Marshal(zt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected null, got %s", string(data))
	}
}

func TestBuildUnmarshal_WithTimestamps(t *testing.T) {
	input := `{
		"uuid": "abc-123",
		"job_name": "test-job",
		"start_time": "2026-03-06T07:24:19",
		"end_time": "2026-03-06T07:30:00Z",
		"pipeline": "check",
		"voting": true,
		"ref": {
			"project": "my-project",
			"branch": "main"
		}
	}`
	var build Build
	if err := json.Unmarshal([]byte(input), &build); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build.UUID != "abc-123" {
		t.Errorf("expected abc-123, got %s", build.UUID)
	}
	if build.StartTime == nil {
		t.Fatal("expected StartTime to be set")
	}
	if build.StartTime.Hour() != 7 {
		t.Errorf("expected hour 7, got %d", build.StartTime.Hour())
	}
	if build.EndTime == nil {
		t.Fatal("expected EndTime to be set")
	}
	if build.EndTime.Minute() != 30 {
		t.Errorf("expected minute 30, got %d", build.EndTime.Minute())
	}
	if build.Ref == nil || build.Ref.Project != "my-project" {
		t.Errorf("expected ref.project = my-project")
	}
}

func TestBuildUnmarshal_NullTimestamps(t *testing.T) {
	input := `{
		"uuid": "abc-123",
		"job_name": "test-job",
		"start_time": null,
		"pipeline": "check",
		"voting": true,
		"ref": {
			"project": "my-project"
		}
	}`
	var build Build
	if err := json.Unmarshal([]byte(input), &build); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build.UUID != "abc-123" {
		t.Errorf("expected abc-123, got %s", build.UUID)
	}
	// StartTime should be nil (pointer is nil) or zero
	if build.StartTime != nil && !build.StartTime.IsZero() {
		t.Errorf("expected nil or zero StartTime, got %v", build.StartTime)
	}
}

func TestRefUnmarshal(t *testing.T) {
	input := `{
		"project": "openstack/nova",
		"branch": "master",
		"change": 978841,
		"patchset": "6",
		"ref": "refs/changes/41/978841/6",
		"oldrev": null,
		"newrev": null,
		"ref_url": "https://review.opendev.org/c/openstack/nova/+/978841"
	}`
	var ref Ref
	if err := json.Unmarshal([]byte(input), &ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Project != "openstack/nova" {
		t.Errorf("expected openstack/nova, got %s", ref.Project)
	}
	if ref.Change != 978841 {
		t.Errorf("expected change 978841, got %d", ref.Change)
	}
	if ref.RefURL != "https://review.opendev.org/c/openstack/nova/+/978841" {
		t.Errorf("expected ref_url, got %s", ref.RefURL)
	}
}

func TestBuildsetUnmarshal_WithRefs(t *testing.T) {
	input := `{
		"uuid": "aed32a12f1454f7599b4eadaad8d8694",
		"result": "FAILURE",
		"message": "Build failed.",
		"pipeline": "check",
		"refs": [
			{
				"project": "openstack/releases",
				"branch": "master",
				"change": 980900,
				"patchset": "2"
			}
		]
	}`
	var bs Buildset
	if err := json.Unmarshal([]byte(input), &bs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bs.UUID != "aed32a12f1454f7599b4eadaad8d8694" {
		t.Errorf("expected uuid aed32a12f1454f7599b4eadaad8d8694, got %s", bs.UUID)
	}
	if len(bs.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(bs.Refs))
	}
	if bs.Refs[0].Project != "openstack/releases" {
		t.Errorf("expected openstack/releases, got %s", bs.Refs[0].Project)
	}
	if bs.Refs[0].Change != 980900 {
		t.Errorf("expected change 980900, got %d", bs.Refs[0].Change)
	}
}
