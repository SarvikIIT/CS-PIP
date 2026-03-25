package analyzer

// ---------------- WORKLOAD TYPE ----------------

type WorkloadType string

const (
	CPUbound    WorkloadType = "CPU-bound"
	MemoryBound WorkloadType = "Memory-bound"
	IOBound     WorkloadType = "I/O-bound"
	Mixed       WorkloadType = "Mixed"
	Idle        WorkloadType = "Idle"
	Unknown     WorkloadType = "Unknown"
)

// ---------------- CONFIDENCE ----------------

type ConfidenceLevel string

const (
	HighConfidence   ConfidenceLevel = "HIGH"
	MediumConfidence ConfidenceLevel = "MEDIUM"
	LowConfidence    ConfidenceLevel = "LOW"
)

// ---------------- BOTTLENECK ----------------

type Bottleneck struct {
	Type     string // cpu_throttling, memory_pressure, io_saturation
	Severity string // CRITICAL, WARNING, INFO
	Detail   string // human-readable explanation
}

// ---------------- PATTERN RESULT ----------------

type PatternResult struct {
	// Memory
	MemoryLeak        bool
	MemoryGrowthBytes uint64

	// CPU
	CPUMean     float64
	CPUStdDev   float64
	IsBurstyCPU bool
	IsSteadyCPU bool

	// IO
	IOSpikeCount int
	IsPeriodicIO bool
}

// ---------------- CLASSIFICATION RESULT ----------------

type Classification struct {
	Type       WorkloadType
	Confidence ConfidenceLevel
	Score      float64 // e.g. 0.93 (93% samples matched)
	Reason     string  // explanation for report
}

// ---------------- FINAL ANALYSIS RESULT ----------------

type AnalysisResult struct {
	Classification Classification
	Bottlenecks    []Bottleneck
	Patterns       PatternResult
}