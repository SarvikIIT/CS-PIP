package reporter

// MetricStats holds aggregated statistics for a single metric.
type MetricStats struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// AlertSeverity describes how critical a performance alert is.
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "CRITICAL"
	SeverityWarning  AlertSeverity = "WARNING"
	SeverityInfo     AlertSeverity = "INFO"
)

// Alert represents a single performance alert or bottleneck notice.
type Alert struct {
	Severity AlertSeverity `json:"severity"`
	Type     string        `json:"type"`
	Detail   string        `json:"detail"`
}

// Classification holds the workload type classification result.
type Classification struct {
	Type       string  `json:"type"`
	Confidence string  `json:"confidence"`
	Score      float64 `json:"score,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// Metrics holds aggregated statistics for all monitored resource metrics.
type Metrics struct {
	CPUPercent    MetricStats `json:"cpu_percent"`
	MemRSSMB      MetricStats `json:"mem_rss_mb"`
	IOReadMBps    MetricStats `json:"io_read_mbps"`
	IOWriteMBps   MetricStats `json:"io_write_mbps"`
	MajFaultsPS   MetricStats `json:"major_faults_ps"`
	CtxSwitchesPS MetricStats `json:"context_switches_ps"`
}

// Report is the complete execution performance report for a container run.
type Report struct {
	ContainerID    string         `json:"container_id"`
	Command        []string       `json:"command"`
	DurationSecs   float64        `json:"duration_seconds"`
	ExitCode       int            `json:"exit_code"`
	Metrics        Metrics        `json:"metrics"`
	Classification Classification `json:"classification"`
	Bottlenecks    []Alert        `json:"bottlenecks"`
	Suggestions    []string       `json:"suggestions"`
}
