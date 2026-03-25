package analyzer_test

import (
	"math"
	"testing"
	"time"

	"github.com/SarvikIIT/CS-PIP/internal/analyzer"
	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

// ------------------------------------------------------------------ helpers

func snapshot(ts time.Time, cpu float64, rss, readB, writeB, majFlt, vol, inv uint64) profiler.ProfileSnapshot {
	return profiler.ProfileSnapshot{
		Timestamp:      ts,
		CPUPercent:     cpu,
		MemRSSBytes:    rss,
		IOReadBytes:    readB,
		IOWriteBytes:   writeB,
		MajorFaults:    majFlt,
		VolCtxSwitches: vol,
		InvCtxSwitches: inv,
	}
}

func makeSeries(n int, cpu float64, rss, writePerStep uint64) profiler.MetricSeries {
	base := time.Now()
	series := make(profiler.MetricSeries, n)
	for i := range series {
		series[i] = snapshot(
			base.Add(time.Duration(i)*100*time.Millisecond),
			cpu,
			rss,
			0,
			writePerStep*uint64(i),
			0, 0, 0,
		)
	}
	return series
}

// ------------------------------------------------------------------ types.go

func TestWorkloadTypeConstants(t *testing.T) {
	if string(analyzer.CPUBound) != "CPU-bound" {
		t.Errorf("CPUBound = %q, want CPU-bound", analyzer.CPUBound)
	}
	if string(analyzer.MemoryBound) != "Memory-bound" {
		t.Errorf("MemoryBound = %q, want Memory-bound", analyzer.MemoryBound)
	}
	if string(analyzer.IOBound) != "I/O-bound" {
		t.Errorf("IOBound = %q, want I/O-bound", analyzer.IOBound)
	}
}

// ------------------------------------------------------------------ classifier.go

func TestClassify_TooFewSamples(t *testing.T) {
	wt, conf := analyzer.Classify(nil, 0)
	if wt != analyzer.Unknown {
		t.Errorf("got %v, want Unknown", wt)
	}
	if conf != analyzer.LowConfidence {
		t.Errorf("got %v, want LowConfidence", conf)
	}

	single := makeSeries(1, 90, 0, 0)
	wt, conf = analyzer.Classify(single, 0)
	if wt != analyzer.Unknown || conf != analyzer.LowConfidence {
		t.Errorf("single sample: got (%v, %v), want (Unknown, LowConfidence)", wt, conf)
	}
}

func TestClassify_CPUBound(t *testing.T) {
	// High CPU, low IO → CPU-bound.
	series := makeSeries(20, 85, 1024*1024, 0)
	wt, _ := analyzer.Classify(series, 512*1024*1024)
	if wt != analyzer.CPUBound {
		t.Errorf("got %v, want CPUBound", wt)
	}
}

func TestClassify_IOBound(t *testing.T) {
	// Low CPU, high IO → I/O-bound.
	// writePerStep of 2 MB means ~20 MB/s IO rate at 100 ms intervals.
	series := makeSeries(20, 10, 1024*1024, 2*1024*1024)
	wt, _ := analyzer.Classify(series, 512*1024*1024)
	if wt != analyzer.IOBound {
		t.Errorf("got %v, want IOBound", wt)
	}
}

func TestClassify_CounterReset_NoUnderflow(t *testing.T) {
	// Simulate a counter reset: second sample has lower IO than first.
	base := time.Now()
	series := profiler.MetricSeries{
		snapshot(base, 20, 0, 0, 1000, 0, 0, 0),
		snapshot(base.Add(100*time.Millisecond), 20, 0, 0, 0, 0, 0, 0), // reset
		snapshot(base.Add(200*time.Millisecond), 20, 0, 0, 500, 0, 0, 0),
	}
	// Should not panic or produce garbage results.
	wt, conf := analyzer.Classify(series, 0)
	if wt == "" || conf == "" {
		t.Error("Classify returned empty result on counter reset input")
	}
}

func TestClassify_MemLimitZero_NoFalseMemBound(t *testing.T) {
	// With memLimit == 0, even large RSS should not trigger Memory-bound via
	// the memLimit threshold (only via major faults).
	series := makeSeries(20, 20, 10*1024*1024*1024, 0) // 10 GiB RSS, 0 limit
	wt, _ := analyzer.Classify(series, 0)
	// Should not be Memory-bound because memLimit is unknown/unlimited.
	if wt == analyzer.MemoryBound {
		t.Errorf("got MemoryBound with memLimit==0, want non-MemoryBound")
	}
}

// ------------------------------------------------------------------ bottleneck.go

func TestDetectBottlenecks_Empty(t *testing.T) {
	// Without root we can't read cgroup files, so only the IO saturation
	// path is exercised here.
	bns := analyzer.DetectBottlenecks(nil, "nonexistent")
	// No panic, any number of bottlenecks is fine (cgroup paths won't exist).
	_ = bns
}

func TestDetectBottlenecks_IOSaturation(t *testing.T) {
	// Build a series where write rate plateaus → should detect IO saturation.
	base := time.Now()
	series := make(profiler.MetricSeries, 10)
	// Constant write rate (no growth → plateau every interval).
	for i := range series {
		series[i] = snapshot(
			base.Add(time.Duration(i)*time.Second),
			20, 0, 0, uint64(i)*1024*1024, 0, 0, 0,
		)
	}
	bns := analyzer.DetectBottlenecks(series, "nonexistent")
	found := false
	for _, b := range bns {
		if b.Type == "I/O Saturation" {
			found = true
		}
	}
	if !found {
		t.Error("expected I/O Saturation bottleneck for plateauing write rate")
	}
}

func TestDetectBottlenecks_CounterReset_NoUnderflow(t *testing.T) {
	// Counter reset in IOWriteBytes should not produce huge rates.
	base := time.Now()
	series := profiler.MetricSeries{
		snapshot(base, 20, 0, 0, 1_000_000_000, 0, 0, 0),
		snapshot(base.Add(time.Second), 20, 0, 0, 0, 0, 0, 0), // reset
		snapshot(base.Add(2*time.Second), 20, 0, 0, 500, 0, 0, 0),
		snapshot(base.Add(3*time.Second), 20, 0, 0, 600, 0, 0, 0),
		snapshot(base.Add(4*time.Second), 20, 0, 0, 700, 0, 0, 0),
	}
	// Must not panic.
	_ = analyzer.DetectBottlenecks(series, "nonexistent")
}

// ------------------------------------------------------------------ pattern.go

func TestAnalyzePatterns_Empty(t *testing.T) {
	r := analyzer.AnalyzePatterns(nil)
	if r.MemoryLeak || r.CPUMean != 0 || r.IOSpikeCount != 0 {
		t.Errorf("unexpected non-zero result for empty series: %+v", r)
	}
}

func TestAnalyzePatterns_MemoryLeak(t *testing.T) {
	// Monotonically increasing RSS → memory leak.
	base := time.Now()
	series := make(profiler.MetricSeries, 10)
	for i := range series {
		series[i] = snapshot(base.Add(time.Duration(i)*100*time.Millisecond),
			10, uint64(i+1)*1024*1024, 0, 0, 0, 0, 0)
	}
	r := analyzer.AnalyzePatterns(series)
	if !r.MemoryLeak {
		t.Error("expected MemoryLeak=true for monotonically increasing RSS")
	}
	if r.MemoryGrowthBytes == 0 {
		t.Error("expected non-zero MemoryGrowthBytes")
	}
}

func TestAnalyzePatterns_IOSpikes_CounterReset(t *testing.T) {
	// Counter reset should not produce bogus spike data.
	base := time.Now()
	series := profiler.MetricSeries{
		snapshot(base, 10, 0, 0, 1_000_000_000, 0, 0, 0),
		snapshot(base.Add(100*time.Millisecond), 10, 0, 0, 0, 0, 0, 0), // reset
		snapshot(base.Add(200*time.Millisecond), 10, 0, 0, 100, 0, 0, 0),
	}
	r := analyzer.AnalyzePatterns(series)
	// The huge counter reset delta should be skipped, so spike count should be 0.
	if r.IOSpikeCount != 0 {
		t.Errorf("got IOSpikeCount=%d after counter reset, want 0", r.IOSpikeCount)
	}
}

// ------------------------------------------------------------------ fingerprints.go

func TestBuildFingerprint_Empty(t *testing.T) {
	fp := analyzer.BuildFingerprint(nil, 0)
	if len(fp.Vector) != 0 {
		t.Errorf("expected empty vector, got %v", fp.Vector)
	}
}

func TestBuildFingerprint_SingleSample(t *testing.T) {
	// Single sample should not divide by zero.
	s := profiler.MetricSeries{
		snapshot(time.Now(), 50, 256*1024*1024, 0, 0, 0, 0, 0),
	}
	fp := analyzer.BuildFingerprint(s, 512*1024*1024)
	if len(fp.Vector) != 5 {
		t.Fatalf("expected 5-element vector, got %d", len(fp.Vector))
	}
	for _, v := range fp.Vector {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("vector contains NaN/Inf: %v", fp.Vector)
			break
		}
	}
	// cpuNorm should be 0.5 for 50%
	if math.Abs(fp.Vector[0]-0.5) > 1e-9 {
		t.Errorf("cpuNorm = %v, want 0.5", fp.Vector[0])
	}
}

func TestBuildFingerprint_NormalCase(t *testing.T) {
	series := makeSeries(10, 80, 256*1024*1024, 0)
	fp := analyzer.BuildFingerprint(series, 512*1024*1024)
	if len(fp.Vector) != 5 {
		t.Fatalf("expected 5-element vector, got %d", len(fp.Vector))
	}
	for _, v := range fp.Vector {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("vector contains NaN/Inf: %v", fp.Vector)
			break
		}
	}
}

func TestBuildFingerprint_CounterReset_NoUnderflow(t *testing.T) {
	base := time.Now()
	series := profiler.MetricSeries{
		snapshot(base, 50, 0, 0, 1_000_000_000, 10, 100, 10),
		snapshot(base.Add(100*time.Millisecond), 50, 0, 0, 0, 0, 0, 0), // all reset
		snapshot(base.Add(200*time.Millisecond), 50, 0, 0, 500, 2, 50, 5),
	}
	fp := analyzer.BuildFingerprint(series, 0)
	for _, v := range fp.Vector {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			t.Errorf("invalid vector value after counter reset: %v", fp.Vector)
			break
		}
	}
}

func TestDistance_SameFingerprint(t *testing.T) {
	fp := analyzer.Fingerprint{Vector: []float64{0.5, 0.3, 0.1, 0.0, 0.0}}
	if d := analyzer.Distance(fp, fp); d != 0 {
		t.Errorf("Distance of identical fingerprints = %v, want 0", d)
	}
}

func TestDistance_DifferentLength(t *testing.T) {
	a := analyzer.Fingerprint{Vector: []float64{0.5}}
	b := analyzer.Fingerprint{Vector: []float64{0.5, 0.3}}
	if d := analyzer.Distance(a, b); d != math.MaxFloat64 {
		t.Errorf("Distance with mismatched lengths = %v, want MaxFloat64", d)
	}
}

func TestIsSimilar(t *testing.T) {
	a := analyzer.Fingerprint{Vector: []float64{0.0, 0.0}}
	b := analyzer.Fingerprint{Vector: []float64{0.1, 0.0}}
	if !analyzer.IsSimilar(a, b, 0.2) {
		t.Error("expected similar for distance 0.1 with threshold 0.2")
	}
	if analyzer.IsSimilar(a, b, 0.05) {
		t.Error("expected not similar for distance 0.1 with threshold 0.05")
	}
}
