package reporter_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/SarvikIIT/CS-PIP/internal/analyzer"
	"github.com/SarvikIIT/CS-PIP/internal/profiler"
	"github.com/SarvikIIT/CS-PIP/internal/reporter"
)

// ------------------------------------------------------------------ helpers

func makeSnapshot(ts time.Time, cpu float64, rssBytes, readB, writeB, majFlt, vol, inv uint64) profiler.ProfileSnapshot {
	return profiler.ProfileSnapshot{
		Timestamp:      ts,
		CPUPercent:     cpu,
		MemRSSBytes:    rssBytes,
		IOReadBytes:    readB,
		IOWriteBytes:   writeB,
		MajorFaults:    majFlt,
		VolCtxSwitches: vol,
		InvCtxSwitches: inv,
	}
}

func makeSeries(n int, cpu float64, rssBytes, writePerStep uint64) []profiler.ProfileSnapshot {
	base := time.Now()
	series := make([]profiler.ProfileSnapshot, n)
	for i := range series {
		series[i] = makeSnapshot(
			base.Add(time.Duration(i)*100*time.Millisecond),
			cpu, rssBytes, 0, writePerStep*uint64(i), 0, 0, 0,
		)
	}
	return series
}

// ------------------------------------------------------------------ summary.go

func TestComputeStats_Empty(t *testing.T) {
	m := reporter.BuildSummary(nil)
	if m.CPUPercent.Min != 0 || m.CPUPercent.Max != 0 {
		t.Errorf("expected zero stats for empty series, got %+v", m.CPUPercent)
	}
}

func TestComputeStats_Single(t *testing.T) {
	series := makeSeries(1, 50, 128*1024*1024, 0)
	m := reporter.BuildSummary(series)
	if m.CPUPercent.Min != 50 || m.CPUPercent.Max != 50 || m.CPUPercent.Avg != 50 {
		t.Errorf("single sample CPU stats wrong: %+v", m.CPUPercent)
	}
	// With a single snapshot there are no deltas → IO stats should be zero.
	if m.IOReadMBps.Max != 0 {
		t.Errorf("expected zero IO for single sample, got %+v", m.IOReadMBps)
	}
}

func TestComputeStats_P95(t *testing.T) {
	// 20 samples with CPU = index value 1..20 (already sorted).
	base := time.Now()
	series := make([]profiler.ProfileSnapshot, 20)
	for i := range series {
		series[i] = makeSnapshot(base.Add(time.Duration(i)*100*time.Millisecond),
			float64(i+1), 0, 0, 0, 0, 0, 0)
	}
	m := reporter.BuildSummary(series)
	// For 20 samples: p95 index = ceil(0.95*20)-1 = 18 → value = 19.
	if m.CPUPercent.P95 != 19 {
		t.Errorf("P95 = %v, want 19", m.CPUPercent.P95)
	}
	if m.CPUPercent.Min != 1 {
		t.Errorf("Min = %v, want 1", m.CPUPercent.Min)
	}
	if m.CPUPercent.Max != 20 {
		t.Errorf("Max = %v, want 20", m.CPUPercent.Max)
	}
}

func TestBuildSummary_IORate(t *testing.T) {
	// 11 snapshots, writePerStep = 1 MB, interval = 100ms → 10 MB/s each step.
	series := makeSeries(11, 80, 256*1024*1024, 1*1024*1024)
	m := reporter.BuildSummary(series)
	// Each delta is 1 MB over 0.1 s = 10 MB/s.
	if m.IOWriteMBps.Avg < 9.9 || m.IOWriteMBps.Avg > 10.1 {
		t.Errorf("IOWriteMBps.Avg = %v, want ~10", m.IOWriteMBps.Avg)
	}
}

func TestBuildSummary_CounterReset(t *testing.T) {
	base := time.Now()
	series := []profiler.ProfileSnapshot{
		makeSnapshot(base, 50, 0, 0, 1_000_000_000, 0, 0, 0),
		makeSnapshot(base.Add(100*time.Millisecond), 50, 0, 0, 0, 0, 0, 0), // reset
		makeSnapshot(base.Add(200*time.Millisecond), 50, 0, 0, 500, 0, 0, 0),
	}
	m := reporter.BuildSummary(series)
	// Counter resets should be treated as zero deltas, not huge values.
	if m.IOWriteMBps.Max > 1 {
		t.Errorf("IOWriteMBps.Max = %v after counter reset, expected near 0", m.IOWriteMBps.Max)
	}
}

// ------------------------------------------------------------------ text_renderer.go

func TestRenderText_ContainsKeyFields(t *testing.T) {
	rpt := reporter.Report{
		ContainerID:  "abc123",
		Command:      []string{"/bin/stress", "--cpu", "4"},
		DurationSecs: 12.4,
		ExitCode:     0,
		Metrics:      reporter.Metrics{},
		Classification: reporter.Classification{
			Type:       "CPU-bound",
			Confidence: "HIGH",
			Score:      0.93,
		},
		Bottlenecks: []reporter.Alert{
			{Severity: reporter.SeverityCritical, Type: "CPU Throttling", Detail: "throttled 42 times"},
		},
		Suggestions: []string{"Increase CPU quota."},
	}

	var buf bytes.Buffer
	reporter.RenderText(rpt, &buf)
	out := buf.String()

	checks := []string{
		"=== CSPIP Execution Report ===",
		"abc123",
		"/bin/stress --cpu 4",
		"12.4s",
		"CPU-bound",
		"HIGH",
		"93%",
		"[CRITICAL]",
		"CPU Throttling",
		"Increase CPU quota.",
		"=== End of Report ===",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("RenderText output missing %q", want)
		}
	}
}

func TestRenderText_NoBottlenecks(t *testing.T) {
	rpt := reporter.Report{
		ContainerID:    "xyz",
		Command:        []string{"/bin/sh"},
		DurationSecs:   1.0,
		Classification: reporter.Classification{Type: "Unknown", Confidence: "LOW"},
	}
	var buf bytes.Buffer
	reporter.RenderText(rpt, &buf)
	if !strings.Contains(buf.String(), "None detected.") {
		t.Error("expected 'None detected.' when no bottlenecks")
	}
}

// ------------------------------------------------------------------ json_renderer.go

func TestRenderJSON_ValidJSON(t *testing.T) {
	rpt := reporter.Report{
		ContainerID:  "abc123",
		Command:      []string{"/bin/sh"},
		DurationSecs: 5.0,
		ExitCode:     0,
		Classification: reporter.Classification{
			Type:       "CPU-bound",
			Confidence: "HIGH",
		},
	}
	var buf bytes.Buffer
	if err := reporter.RenderJSON(rpt, &buf); err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}

	var decoded reporter.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}
	if decoded.ContainerID != "abc123" {
		t.Errorf("decoded ContainerID = %q, want abc123", decoded.ContainerID)
	}
	if decoded.Classification.Type != "CPU-bound" {
		t.Errorf("decoded Classification.Type = %q, want CPU-bound", decoded.Classification.Type)
	}
}

// ------------------------------------------------------------------ alert.go

func TestGenerateAlerts_Empty(t *testing.T) {
	result := analyzer.AnalysisResult{}
	alerts := reporter.GenerateAlerts(nil, result, 0)
	// Should not panic; empty input → no alerts from patterns.
	_ = alerts
}

func TestGenerateAlerts_MemoryNearLimit(t *testing.T) {
	// RSS is 95% of limit → should fire memory_near_limit.
	limit := uint64(256 * 1024 * 1024)
	rss := uint64(float64(limit) * 0.95)
	series := []profiler.ProfileSnapshot{
		makeSnapshot(time.Now(), 50, rss, 0, 0, 0, 0, 0),
	}
	result := analyzer.AnalysisResult{}
	alerts := reporter.GenerateAlerts(series, result, limit)
	found := false
	for _, a := range alerts {
		if a.Type == "memory_near_limit" {
			found = true
			if a.Severity != reporter.SeverityWarning {
				t.Errorf("memory_near_limit severity = %v, want WARNING", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected memory_near_limit alert")
	}
}

func TestGenerateAlerts_MemoryBelowLimit(t *testing.T) {
	// RSS is 50% of limit → no memory_near_limit.
	limit := uint64(256 * 1024 * 1024)
	rss := limit / 2
	series := []profiler.ProfileSnapshot{
		makeSnapshot(time.Now(), 50, rss, 0, 0, 0, 0, 0),
	}
	result := analyzer.AnalysisResult{}
	alerts := reporter.GenerateAlerts(series, result, limit)
	for _, a := range alerts {
		if a.Type == "memory_near_limit" {
			t.Error("unexpected memory_near_limit alert when RSS is well below limit")
		}
	}
}

func TestGenerateAlerts_MemoryLeak(t *testing.T) {
	result := analyzer.AnalysisResult{
		Patterns: analyzer.PatternResult{
			MemoryLeak:        true,
			MemoryGrowthBytes: 50 * 1024 * 1024,
		},
	}
	alerts := reporter.GenerateAlerts(nil, result, 0)
	found := false
	for _, a := range alerts {
		if a.Type == "memory_leak" {
			found = true
		}
	}
	if !found {
		t.Error("expected memory_leak alert")
	}
}

func TestGenerateAlerts_BurstyCPU(t *testing.T) {
	result := analyzer.AnalysisResult{
		Patterns: analyzer.PatternResult{
			IsBurstyCPU: true,
			CPUMean:     80,
			CPUStdDev:   30,
		},
	}
	alerts := reporter.GenerateAlerts(nil, result, 0)
	found := false
	for _, a := range alerts {
		if a.Type == "bursty_cpu" {
			found = true
		}
	}
	if !found {
		t.Error("expected bursty_cpu alert")
	}
}

func TestGenerateSuggestions_Dedup(t *testing.T) {
	// Two alerts of the same type should not produce duplicate suggestions.
	alerts := []reporter.Alert{
		{Severity: reporter.SeverityCritical, Type: "CPU Throttling", Detail: "..."},
		{Severity: reporter.SeverityCritical, Type: "CPU Throttling", Detail: "..."},
	}
	cls := reporter.Classification{Type: "CPU-bound"}
	suggestions := reporter.GenerateSuggestions(alerts, cls)
	seen := make(map[string]int)
	for _, s := range suggestions {
		seen[s]++
		if seen[s] > 1 {
			t.Errorf("duplicate suggestion: %q", s)
		}
	}
}

func TestGenerateSuggestions_Fallback(t *testing.T) {
	// No alerts → fallback suggestion based on classification.
	cls := reporter.Classification{Type: "I/O-bound"}
	suggestions := reporter.GenerateSuggestions(nil, cls)
	if len(suggestions) == 0 {
		t.Error("expected at least one fallback suggestion for I/O-bound with no alerts")
	}
}
