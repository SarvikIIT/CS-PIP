package reporter

import (
	"math"
	"sort"

	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

const bytesToMB = 1024 * 1024

// computeStats calculates min, average, 95th-percentile, and max over a
// slice of float64 values.  Returns a zero-valued MetricStats for empty input.
func computeStats(values []float64) MetricStats {
	n := len(values)
	if n == 0 {
		return MetricStats{}
	}

	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}

	// 95th-percentile index: ceil(0.95 * n) - 1, clamped to valid range.
	p95idx := int(math.Ceil(0.95*float64(n))) - 1
	if p95idx < 0 {
		p95idx = 0
	}
	if p95idx >= n {
		p95idx = n - 1
	}

	return MetricStats{
		Min: sorted[0],
		Avg: sum / float64(n),
		P95: sorted[p95idx],
		Max: sorted[n-1],
	}
}

// BuildSummary aggregates a MetricSeries into per-metric statistics.
// Rate metrics (IO, page faults, context switches) are computed as
// deltas between consecutive snapshots divided by elapsed time.
func BuildSummary(series []profiler.ProfileSnapshot) Metrics {
	n := len(series)
	if n == 0 {
		return Metrics{}
	}

	cpuVals := make([]float64, 0, n)
	memVals := make([]float64, 0, n)
	ioReadVals := make([]float64, 0, n)
	ioWriteVals := make([]float64, 0, n)
	majFaultVals := make([]float64, 0, n)
	ctxVals := make([]float64, 0, n)

	for i, snap := range series {
		cpuVals = append(cpuVals, snap.CPUPercent)
		memVals = append(memVals, float64(snap.MemRSSBytes)/bytesToMB) // bytes → MB

		if i == 0 {
			continue
		}

		prev := series[i-1]
		dt := snap.Timestamp.Sub(prev.Timestamp).Seconds()
		if dt <= 0 {
			continue
		}

		// IO rates (MB/s) — guard against counter reset (curr < prev).
		var readDelta, writeDelta uint64
		if snap.IOReadBytes >= prev.IOReadBytes {
			readDelta = snap.IOReadBytes - prev.IOReadBytes
		}
		if snap.IOWriteBytes >= prev.IOWriteBytes {
			writeDelta = snap.IOWriteBytes - prev.IOWriteBytes
		}
		ioReadVals = append(ioReadVals, float64(readDelta)/dt/bytesToMB)
		ioWriteVals = append(ioWriteVals, float64(writeDelta)/dt/bytesToMB)

		// Major page fault rate (/s) — guard against counter reset.
		var majDelta uint64
		if snap.MajorFaults >= prev.MajorFaults {
			majDelta = snap.MajorFaults - prev.MajorFaults
		}
		majFaultVals = append(majFaultVals, float64(majDelta)/dt)

		// Context switch rate (/s) — guard against counter reset.
		var volDelta, invDelta uint64
		if snap.VolCtxSwitches >= prev.VolCtxSwitches {
			volDelta = snap.VolCtxSwitches - prev.VolCtxSwitches
		}
		if snap.InvCtxSwitches >= prev.InvCtxSwitches {
			invDelta = snap.InvCtxSwitches - prev.InvCtxSwitches
		}
		ctxVals = append(ctxVals, float64(volDelta+invDelta)/dt)
	}

	return Metrics{
		CPUPercent:    computeStats(cpuVals),
		MemRSSMB:      computeStats(memVals),
		IOReadMBps:    computeStats(ioReadVals),
		IOWriteMBps:   computeStats(ioWriteVals),
		MajFaultsPS:   computeStats(majFaultVals),
		CtxSwitchesPS: computeStats(ctxVals),
	}
}
