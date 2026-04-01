package reporter

import (
	"fmt"

	"github.com/SarvikIIT/CS-PIP/internal/analyzer"
	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

// alert thresholds
const (
	memNearLimitRatio    = 0.90 // warn when RSS > 90% of memory limit
	highMajFaultRate     = 50.0 // warn when avg major faults/s > 50
)

// GenerateAlerts produces a list of Alerts from profiling data and analysis
// results.  It converts analyzer Bottlenecks to Alerts and adds pattern-based
// alerts (memory leak, bursty CPU, memory near limit).
func GenerateAlerts(
	series []profiler.ProfileSnapshot,
	result analyzer.AnalysisResult,
	memLimitBytes uint64,
) []Alert {
	var alerts []Alert

	// Convert analyzer Bottlenecks → reporter Alerts.
	for _, bn := range result.Bottlenecks {
		sev := SeverityWarning
		if bn.Severity == analyzer.SeverityHigh {
			sev = SeverityCritical
		}
		alerts = append(alerts, Alert{
			Severity: sev,
			Type:     bn.Type,
			Detail:   bn.Detail,
		})
	}

	// Memory near limit.
	if memLimitBytes > 0 && len(series) > 0 {
		lastRSS := series[len(series)-1].MemRSSBytes
		if float64(lastRSS) > memNearLimitRatio*float64(memLimitBytes) {
			alerts = append(alerts, Alert{
				Severity: SeverityWarning,
				Type:     "memory_near_limit",
				Detail: fmt.Sprintf(
					"RSS %.1f MB is near memory limit %.1f MB",
					float64(lastRSS)/(1024*1024),
					float64(memLimitBytes)/(1024*1024),
				),
			})
		}
	}

	// High major page fault rate.
	metrics := BuildSummary(series)
	if metrics.MajFaultsPS.Avg > highMajFaultRate {
		alerts = append(alerts, Alert{
			Severity: SeverityWarning,
			Type:     "high_major_faults",
			Detail: fmt.Sprintf(
				"average major fault rate %.1f/s exceeds threshold (%.0f/s)",
				metrics.MajFaultsPS.Avg, highMajFaultRate,
			),
		})
	}

	// Memory leak.
	if result.Patterns.MemoryLeak {
		growthMB := float64(result.Patterns.MemoryGrowthBytes) / (1024 * 1024)
		alerts = append(alerts, Alert{
			Severity: SeverityWarning,
			Type:     "memory_leak",
			Detail:   fmt.Sprintf("RSS grew monotonically by %.1f MB over the run", growthMB),
		})
	}

	// Bursty CPU pattern.
	if result.Patterns.IsBurstyCPU && result.Patterns.CPUMean > 0 {
		ratio := result.Patterns.CPUStdDev / result.Patterns.CPUMean * 100
		alerts = append(alerts, Alert{
			Severity: SeverityWarning,
			Type:     "bursty_cpu",
			Detail: fmt.Sprintf(
				"CPU stddev %.1f%% is %.0f%% of mean — bursty usage pattern",
				result.Patterns.CPUStdDev, ratio,
			),
		})
	}

	return alerts
}

// GenerateSuggestions returns actionable optimization suggestions derived
// from the alert list and workload classification.
func GenerateSuggestions(alerts []Alert, classification Classification) []string {
	var suggestions []string

	for _, a := range alerts {
		switch a.Type {
		case "CPU Throttling":
			suggestions = append(suggestions,
				"Increase the CPU quota (--cpu-limit) or reduce the number of worker threads.")
			suggestions = append(suggestions,
				"Consider spreading CPU-intensive work across time to reduce peak usage.")
		case "Memory Pressure":
			suggestions = append(suggestions,
				"Increase the memory limit (--mem-limit) or optimize memory usage in the workload.")
		case "memory_near_limit":
			suggestions = append(suggestions,
				"Increase the memory limit (--mem-limit) to provide more headroom.")
		case "high_major_faults":
			suggestions = append(suggestions,
				"Reduce working set size or increase memory allocation to avoid major page faults.")
		case "memory_leak":
			suggestions = append(suggestions,
				"Investigate allocation patterns to find and fix the memory leak.")
		case "I/O Saturation":
			suggestions = append(suggestions,
				"Consider buffering writes or using async I/O to reduce I/O saturation.")
		case "bursty_cpu":
			suggestions = append(suggestions,
				"Smooth out CPU bursts with rate limiting or work queues.")
		}
	}

	// Classification-based fallback suggestions when no alerts fired.
	if len(suggestions) == 0 {
		switch classification.Type {
		case "CPU-bound":
			suggestions = append(suggestions,
				"Consider parallelizing the workload or distributing across more CPU cores.")
		case "Memory-bound":
			suggestions = append(suggestions,
				"Optimize data structures to reduce memory footprint.")
		case "I/O-bound":
			suggestions = append(suggestions,
				"Consider caching frequently accessed data to reduce I/O operations.")
		}
	}

	// Deduplicate while preserving order.
	seen := make(map[string]bool, len(suggestions))
	deduped := suggestions[:0]
	for _, s := range suggestions {
		if !seen[s] {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}

	return deduped
}
