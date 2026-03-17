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
		"project": "my-project",
		"pipeline": "check",
		"voting": true
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
}

func TestBuildUnmarshal_NullTimestamps(t *testing.T) {
	input := `{
		"uuid": "abc-123",
		"job_name": "test-job",
		"start_time": null,
		"project": "my-project",
		"pipeline": "check",
		"voting": true
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
