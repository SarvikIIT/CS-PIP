package analyzer

// ---------------- WORKLOAD TYPE ----------------

type WorkloadType string

const (
	CPUBound    WorkloadType = "CPU-bound"
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

// SeverityLevel describes how serious a detected bottleneck is.
type SeverityLevel string

const (
	SeverityHigh   SeverityLevel = "high"
	SeverityMedium SeverityLevel = "medium"
	SeverityLow    SeverityLevel = "low"
)

type Bottleneck struct {
	Type     string        // cpu_throttling, memory_pressure, io_saturation
	Severity SeverityLevel // SeverityHigh, SeverityMedium, SeverityLow
	Detail   string        // human-readable explanation
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