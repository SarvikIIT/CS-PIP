package analyzer

import (
	"math"

	"cspip/internal/profiler"
)

// ---------------- FINGERPRINT STRUCT ----------------

type Fingerprint struct {
	Vector []float64
}

// ---------------- BUILD FINGERPRINT ----------------

func BuildFingerprint(series profiler.MetricSeries, memLimit uint64) Fingerprint {
	n := len(series)
	if n == 0 {
		return Fingerprint{Vector: []float64{}}
	}

	var sumCPU float64
	var sumMem float64
	var sumIO float64
	var sumCtx float64
	var sumFault float64

	for i := 1; i < n; i++ {
		prev := series[i-1]
		curr := series[i]

		// CPU
		sumCPU += curr.CPUPercent

		// Memory (normalized later)
		sumMem += float64(curr.MemRSSBytes)

		// IO delta
		ioDelta := float64(
			(curr.IOReadBytes - prev.IOReadBytes) +
				(curr.IOWriteBytes - prev.IOWriteBytes),
		)
		sumIO += ioDelta

		// Context switches delta
		ctxDelta := float64(
			(curr.VolCtxSwitches - prev.VolCtxSwitches) +
				(curr.InvCtxSwitches - prev.InvCtxSwitches),
		)
		sumCtx += ctxDelta

		// Page faults delta
		faultDelta := float64(
			(curr.MajorFaults - prev.MajorFaults),
		)
		sumFault += faultDelta
	}

	count := float64(n - 1)

	avgCPU := sumCPU / count
	avgMem := sumMem / float64(n)
	avgIO := sumIO / count
	avgCtx := sumCtx / count
	avgFault := sumFault / count

	// ---------------- NORMALIZATION ----------------

	var memNorm float64
	if memLimit > 0 {
		memNorm = avgMem / float64(memLimit)
	}

	// simple normalization (can be improved later)
	cpuNorm := avgCPU / 100.0
	ioNorm := avgIO / (1024 * 1024) // convert to MB scale
	ctxNorm := avgCtx / 1000.0
	faultNorm := avgFault / 100.0

	vector := []float64{
		cpuNorm,
		memNorm,
		ioNorm,
		ctxNorm,
		faultNorm,
	}

	return Fingerprint{Vector: vector}
}

// ---------------- DISTANCE METRIC ----------------

func Distance(a, b Fingerprint) float64 {
	if len(a.Vector) != len(b.Vector) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a.Vector {
		diff := a.Vector[i] - b.Vector[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// ---------------- SIMILARITY CHECK ----------------

func IsSimilar(a, b Fingerprint, threshold float64) bool {
	return Distance(a, b) < threshold
}