package analyzer

import (
	"math"

	"github.com/SarvikIIT/CS-PIP/internal/profiler"
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

	// Special case: a single sample has no deltas; compute from the snapshot.
	if n == 1 {
		sample := series[0]
		avgCPU := sample.CPUPercent
		avgMem := float64(sample.MemRSSBytes)
		// No deltas available for IO, ctx, or faults.
		var memNorm float64
		if memLimit > 0 {
			memNorm = avgMem / float64(memLimit)
		}
		return Fingerprint{Vector: []float64{
			avgCPU / 100.0,
			memNorm,
			0, // IO
			0, // context switches
			0, // page faults
		}}
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

		// Memory (normalized later); include every sample from index 1 onwards,
		// so the divisor is count (n-1) — consistent with the other averages.
		sumMem += float64(curr.MemRSSBytes)

		// IO delta with reset protection: if a counter decreases (sampling
		// error sets it to 0), treat its delta as 0 to avoid uint64 underflow.
		var readDelta, writeDelta uint64
		if curr.IOReadBytes >= prev.IOReadBytes {
			readDelta = curr.IOReadBytes - prev.IOReadBytes
		}
		if curr.IOWriteBytes >= prev.IOWriteBytes {
			writeDelta = curr.IOWriteBytes - prev.IOWriteBytes
		}
		sumIO += float64(readDelta + writeDelta)

		// Context switches delta (guard against counter resets/underflow).
		var volDelta, invDelta uint64
		if curr.VolCtxSwitches >= prev.VolCtxSwitches {
			volDelta = curr.VolCtxSwitches - prev.VolCtxSwitches
		}
		if curr.InvCtxSwitches >= prev.InvCtxSwitches {
			invDelta = curr.InvCtxSwitches - prev.InvCtxSwitches
		}
		sumCtx += float64(volDelta + invDelta)

		// Page faults delta; guard against counter reset (curr < prev).
		if curr.MajorFaults >= prev.MajorFaults {
			sumFault += float64(curr.MajorFaults - prev.MajorFaults)
		}
	}

	count := float64(n - 1)

	avgCPU := sumCPU / count
	avgMem := sumMem / count // divide by count (n-1), not n
	avgIO := sumIO / count
	avgCtx := sumCtx / count
	avgFault := sumFault / count

	// ---------------- NORMALIZATION ----------------

	var memNorm float64
	if memLimit > 0 {
		memNorm = avgMem / float64(memLimit)
	}

	cpuNorm := avgCPU / 100.0
	ioNorm := avgIO / (1024 * 1024) // convert to MB scale
	ctxNorm := avgCtx / 1000.0
	faultNorm := avgFault / 100.0

	return Fingerprint{Vector: []float64{
		cpuNorm,
		memNorm,
		ioNorm,
		ctxNorm,
		faultNorm,
	}}
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
