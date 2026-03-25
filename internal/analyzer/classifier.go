package analyzer

import (
	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

// Classify inspects a series of profile snapshots and returns the dominant
// WorkloadType together with a ConfidenceLevel.
func Classify(series []profiler.ProfileSnapshot, memLimit uint64) (WorkloadType, ConfidenceLevel) {
	n := len(series)
	if n < 2 {
		return Unknown, LowConfidence
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

		// IO rate (bytes/sec). Guard against counter reset / sampling errors
		// where counters may drop to 0, which would underflow uint64.
		currTotal := curr.IOReadBytes + curr.IOWriteBytes
		prevTotal := prev.IOReadBytes + prev.IOWriteBytes
		var ioRate float64
		if currTotal >= prevTotal {
			ioRate = float64(currTotal-prevTotal) / dt
		}
		totalIO += ioRate

		// Major faults rate. Guard against counter reset (set to 0 on error).
		var deltaMaj uint64
		if curr.MajorFaults >= prev.MajorFaults {
			deltaMaj = curr.MajorFaults - prev.MajorFaults
		}
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
		// Only compare against memLimit when it is meaningful (non-zero).
		if memLimit > 0 && float64(curr.MemRSSBytes) > 0.8*float64(memLimit) {
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
	var workload WorkloadType
	var matchCount int

	switch {
	case avgCPU > 70 && avgIO < 1*1024*1024:
		workload = CPUBound
		matchCount = min(cpuHighCount, ioLowCount)

	case avgMajflt > 100 || (memLimit > 0 && avgRSS > 0.8*float64(memLimit)):
		workload = MemoryBound
		matchCount = max(majfltHighCount, memHighCount)

	case avgIO > 10*1024*1024 || avgCPU < 30:
		workload = IOBound
		matchCount = max(ioHighCount, cpuLowCount)

	case avgCPU > 50 && avgIO > 5*1024*1024:
		workload = Mixed

	default:
		workload = Unknown
	}

	// --- Confidence ---
	confidence := computeConfidence(matchCount, int(samples))

	return workload, confidence
}

func computeConfidence(count int, total int) ConfidenceLevel {
	if total == 0 {
		return LowConfidence
	}

	ratio := float64(count) / float64(total)

	if ratio >= 0.9 {
		return HighConfidence
	} else if ratio >= 0.6 {
		return MediumConfidence
	}
	return LowConfidence
}
