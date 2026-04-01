package reporter

import (
	"fmt"
	"io"
	"strings"
)

// RenderText writes a human-readable performance report to w.
func RenderText(r Report, w io.Writer) {
	fmt.Fprintln(w, "=== CSPIP Execution Report ===")
	fmt.Fprintf(w, "Container ID  : %s\n", r.ContainerID)
	fmt.Fprintf(w, "Command       : %s\n", strings.Join(r.Command, " "))
	fmt.Fprintf(w, "Duration      : %.1fs\n", r.DurationSecs)
	fmt.Fprintf(w, "Exit Code     : %d\n", r.ExitCode)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Resource Usage ---")
	fmt.Fprintf(w, "%-28s%-10s%-10s%-10s%-10s\n", "Metric", "Min", "Avg", "P95", "Max")
	renderMetricPct(w, "CPU %", r.Metrics.CPUPercent)
	renderMetricF(w, "Memory RSS (MB)", r.Metrics.MemRSSMB, "%.0f")
	renderMetricF(w, "Major Page Faults/s", r.Metrics.MajFaultsPS, "%.1f")
	renderMetricF(w, "Context Switches/s", r.Metrics.CtxSwitchesPS, "%.0f")
	renderMetricF(w, "I/O Read (MB/s)", r.Metrics.IOReadMBps, "%.2f")
	renderMetricF(w, "I/O Write (MB/s)", r.Metrics.IOWriteMBps, "%.2f")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Workload Classification ---")
	fmt.Fprintf(w, "Type        : %s\n", r.Classification.Type)
	if r.Classification.Score > 0 {
		fmt.Fprintf(w, "Confidence  : %s (%.0f%% of samples exceeded threshold)\n",
			r.Classification.Confidence, r.Classification.Score*100)
	} else {
		fmt.Fprintf(w, "Confidence  : %s\n", r.Classification.Confidence)
	}
	if r.Classification.Reason != "" {
		fmt.Fprintf(w, "Reasoning   : %s\n", r.Classification.Reason)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Bottlenecks Detected ---")
	if len(r.Bottlenecks) == 0 {
		fmt.Fprintln(w, "None detected.")
	} else {
		for _, a := range r.Bottlenecks {
			fmt.Fprintf(w, "[%s] %s: %s\n", a.Severity, a.Type, a.Detail)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Optimization Suggestions ---")
	if len(r.Suggestions) == 0 {
		fmt.Fprintln(w, "No specific suggestions.")
	} else {
		for _, s := range r.Suggestions {
			fmt.Fprintf(w, "[*] %s\n", s)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== End of Report ===")
}

func renderMetricPct(w io.Writer, name string, stats MetricStats) {
	fmt.Fprintf(w, "%-28s%-10s%-10s%-10s%-10s\n",
		name,
		fmt.Sprintf("%.1f%%", stats.Min),
		fmt.Sprintf("%.1f%%", stats.Avg),
		fmt.Sprintf("%.1f%%", stats.P95),
		fmt.Sprintf("%.1f%%", stats.Max),
	)
}

func renderMetricF(w io.Writer, name string, stats MetricStats, format string) {
	fmt.Fprintf(w, "%-28s%-10s%-10s%-10s%-10s\n",
		name,
		fmt.Sprintf(format, stats.Min),
		fmt.Sprintf(format, stats.Avg),
		fmt.Sprintf(format, stats.P95),
		fmt.Sprintf(format, stats.Max),
	)
}
