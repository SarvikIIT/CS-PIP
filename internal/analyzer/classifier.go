package analyzer

import (
	"math"

	"github.com/yourmodule/internal/profiler"
)

func Classify(series []profiler.ProfileSnapshot, memLimit uint64) (string, string) {
	n := len(series)
	if n < 2 {
		return "Undetermined", "LOW"
	}

	var totalCPU float64
	var totalRSS float64
	var totalIO float64
	var totalMajflt float64

	// For confidence counting
	var cpuHighCount int
	var ioLowCount int
	var memHighCount int
	var majfltHighCount int
	var ioHighCount int
	var cpuLowCount int

	for i := 1; i < n; i++ {
		prev := series[i-1]
		curr := series[i]

		dt := curr.Timestamp.Sub(prev.Timestamp).Seconds()
		if dt == 0 {
			continue
		}

		// CPU (already percentage)
		totalCPU += curr.CPUPercent

		// RSS
		totalRSS += float64(curr.MemRSSBytes)

		// IO rate (bytes/sec)
		deltaIO := (curr.IOReadBytes + curr.IOWriteBytes) -
			(prev.IOReadBytes + prev.IOWriteBytes)
		ioRate := float64(deltaIO) / dt
		totalIO += ioRate

		// Major faults rate
		deltaMaj := curr.MajorFaults - prev.MajorFaults
		majRate := float64(deltaMaj) / dt
		totalMajflt += majRate

		// --- Threshold checks for confidence ---

		if curr.CPUPercent > 70 {
			cpuHighCount++
		}
		if ioRate < 1*1024*1024 {
			ioLowCount++
		}
		if majRate > 100 {
			majfltHighCount++
		}
		if float64(curr.MemRSSBytes) > 0.8*float64(memLimit) {
			memHighCount++
		}
		if ioRate > 10*1024*1024 {
			ioHighCount++
		}
		if curr.CPUPercent < 30 {
			cpuLowCount++
		}
	}

	samples := float64(n - 1)

	avgCPU := totalCPU / samples
	avgRSS := totalRSS / samples
	avgIO := totalIO / samples
	avgMajflt := totalMajflt / samples

	// --- Classification ---
	var workload string
	var matchCount int

	switch {
	case avgCPU > 70 && avgIO < 1*1024*1024:
		workload = "CPU-bound"
		matchCount = min(cpuHighCount, ioLowCount)

	case avgMajflt > 100 || avgRSS > 0.8*float64(memLimit):
		workload = "Memory-bound"
		matchCount = max(majfltHighCount, memHighCount)

	case avgIO > 10*1024*1024 || avgCPU < 30:
		workload = "I/O-bound"
		matchCount = max(ioHighCount, cpuLowCount)

	case avgCPU > 50 && avgIO > 5*1024*1024:
		workload = "Mixed"

	default:
		workload = "Undetermined"
	}

	// --- Confidence ---
	confidence := computeConfidence(matchCount, int(samples))

	return workload, confidence
}

func computeConfidence(count int, total int) string {
	if total == 0 {
		return "LOW"
	}

	ratio := float64(count) / float64(total)

	if ratio >= 0.9 {
		return "HIGH"
	} else if ratio >= 0.6 {
		return "MEDIUM"
	}
	return "LOW"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}