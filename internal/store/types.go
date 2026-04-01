package store

import "github.com/SarvikIIT/CS-PIP/internal/profiler"

// RunRecord holds a container's complete execution record, including all
// profile snapshots collected during the run.  It is persisted to disk so
// that `cspip report` can generate reports after the container has exited.
type RunRecord struct {
	ContainerID   string                     `json:"container_id"`
	Command       []string                   `json:"command"`
	DurationSecs  float64                    `json:"duration_seconds"`
	ExitCode      int                        `json:"exit_code"`
	CPULimit      string                     `json:"cpu_limit,omitempty"`
	MemLimit      string                     `json:"mem_limit,omitempty"`
	MemLimitBytes uint64                     `json:"mem_limit_bytes,omitempty"`
	Series        []profiler.ProfileSnapshot `json:"series"`
}
