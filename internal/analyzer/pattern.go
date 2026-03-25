package analyzer

import (
	"math"

	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

// ---------------- MEMORY LEAK DETECTION ----------------

func detectMemoryLeak(series profiler.MetricSeries) (bool, uint64) {
	n := len(series)
	if n < 5 {
		return false, 0
	}

	indices := []int{
		0,
		n / 4,
		n / 2,
		3 * n / 4,
		n - 1,
	}

	prev := series[indices[0]].MemRSSBytes
	for _, idx := range indices[1:] {
		curr := series[idx].MemRSSBytes
		if curr < prev {
			return false, 0
		}
		prev = curr
	}

	growth := series[n-1].MemRSSBytes - series[0].MemRSSBytes
	return true, growth
}

// ---------------- CPU PATTERN ----------------

func analyzeCPU(series profiler.MetricSeries) (mean, stddev float64, bursty, steady bool) {
	n := len(series)
	if n == 0 {
		return 0, 0, false, false
	}

	var sum float64
	for _, s := range series {
		sum += s.CPUPercent
	}

	mean = sum / float64(n)

	var variance float64
	for _, s := range series {
		diff := s.CPUPercent - mean
		variance += diff * diff
	}

	variance /= float64(n)
	stddev = math.Sqrt(variance)

	if mean > 0 {
		ratio := stddev / mean
		bursty = ratio > 0.3
		steady = ratio < 0.1
	}

	return
}

// ---------------- IO SPIKE DETECTION ----------------

func analyzeIOSpikes(series profiler.MetricSeries) (int, bool) {
	n := len(series)
	if n < 3 {
		return 0, false
	}

	deltas := make([]uint64, 0, n-1)

	for i := 1; i < n; i++ {
		curr := series[i].IOWriteBytes
		prev := series[i-1].IOWriteBytes
		// Guard against counter reset or sampling error (sampler sets bytes
		// to 0 on read error), which would underflow uint64.
		if curr < prev {
			continue
		}
		deltas = append(deltas, curr-prev)
	}

	if len(deltas) == 0 {
		return 0, false
	}

	// median
	sorted := make([]uint64, len(deltas))
	copy(sorted, deltas)

	// simple sort (n small, ok)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	median := sorted[len(sorted)/2]

	if median == 0 {
		return 0, false
	}

	spikeIndices := []int{}

	for i, d := range deltas {
		if float64(d) > 3*float64(median) {
			spikeIndices = append(spikeIndices, i)
		}
	}

	if len(spikeIndices) < 2 {
		return len(spikeIndices), false
	}

	// check periodic spacing
	intervals := []int{}
	for i := 1; i < len(spikeIndices); i++ {
		intervals = append(intervals, spikeIndices[i]-spikeIndices[i-1])
	}

	var sum int
	for _, v := range intervals {
		sum += v
	}

	avg := float64(sum) / float64(len(intervals))

	var variance float64
	for _, v := range intervals {
		diff := float64(v) - avg
		variance += diff * diff
	}

	variance /= float64(len(intervals))
	stddev := math.Sqrt(variance)

	periodic := stddev < 0.2*avg

	return len(spikeIndices), periodic
}

// ---------------- MAIN ENTRY ----------------

func AnalyzePatterns(series profiler.MetricSeries) PatternResult {
	var result PatternResult

	if len(series) == 0 {
		return result
	}

	result.MemoryLeak, result.MemoryGrowthBytes = detectMemoryLeak(series)

	result.CPUMean, result.CPUStdDev, result.IsBurstyCPU, result.IsSteadyCPU =
		analyzeCPU(series)

	result.IOSpikeCount, result.IsPeriodicIO =
		analyzeIOSpikes(series)

	return result
}
