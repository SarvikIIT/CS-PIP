package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SarvikIIT/CS-PIP/internal/analyzer"
	"github.com/SarvikIIT/CS-PIP/internal/profiler"
	"github.com/SarvikIIT/CS-PIP/internal/reporter"
	"github.com/SarvikIIT/CS-PIP/internal/store"
)

// ------------------------------------------------------------------ usage

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: cspip <command> [options]

Commands:
  run   [--cpu-limit <pct>] [--mem-limit <size>] <rootfs> <cmd> [args...]
           Create and run a container (profiles the workload automatically).
           --cpu-limit  CPU limit, e.g. '50%%' or '0.5'
           --mem-limit  Memory limit, e.g. '256m', '1g'

  exec  <container-id> <cmd> [args...]
           Run a command inside a running container.

  inspect <container-id>
           Print container state as JSON.

  ps      List all containers.

  stop    <container-id>   Send SIGTERM and wait for exit.
  kill    <container-id>   Send SIGKILL immediately.
  rm      <container-id>   Remove a stopped container.

  report  <container-id> [--format text|json] [--output <file>]
           Generate a performance report for a completed run.
           --format  Output format: 'text' (default) or 'json'
           --output  Write report to a file instead of stdout

Note: run/exec/inspect/ps/stop/kill/rm require root (sudo).
`)
}

// ------------------------------------------------------------------ helpers

// runtimeBin returns the path to the cspip-runtime C binary.  It first looks
// next to the current executable, then falls back to PATH lookup.
func runtimeBin() string {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), "cspip-runtime")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "cspip-runtime"
}

// delegateToRuntime forwards the given arguments to the C runtime binary and
// returns its exit code.
func delegateToRuntime(args []string) int {
	bin := runtimeBin()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "cspip: runtime error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: ensure 'cspip-runtime' is installed in the same directory as 'cspip'\n")
		return 1
	}
	return 0
}

// parseMemLimit converts a human-readable memory limit string (e.g. "256m",
// "1g") to bytes.  Returns 0 for empty or unrecognised input.
func parseMemLimit(s string) uint64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	var mul uint64 = 1
	switch {
	case strings.HasSuffix(s, "g"):
		mul = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		mul = 1024 * 1024
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "k"):
		mul = 1024
		s = s[:len(s)-1]
	}
	v, _ := strconv.ParseUint(s, 10, 64)
	return v * mul
}

// ------------------------------------------------------------------ run command

// cmdRun starts the C runtime for container creation while simultaneously
// profiling the container process with the Go profiler.  The run record is
// saved to ~/.cspip/runs/<id>.json on completion.
func cmdRun(args []string) int {
	// Parse optional flags before positional arguments.
	var cpuLimit, memLimit string
	i := 1 // args[0] == "run"
	for ; i < len(args); i++ {
		switch args[i] {
		case "--cpu-limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --cpu-limit requires an argument\n")
				return 1
			}
			i++
			cpuLimit = args[i]
		case "--mem-limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --mem-limit requires an argument\n")
				return 1
			}
			i++
			memLimit = args[i]
		default:
			goto doneFlags
		}
	}
doneFlags:

	if i >= len(args) {
		fmt.Fprintf(os.Stderr, "error: missing <rootfs> argument\n")
		usage()
		return 1
	}
	rootfs := args[i]
	i++
	if i >= len(args) {
		fmt.Fprintf(os.Stderr, "error: missing <command> argument\n")
		usage()
		return 1
	}
	cmdSlice := args[i:]

	// Build arguments for the C runtime binary.
	runtimeArgs := []string{"run"}
	if cpuLimit != "" {
		runtimeArgs = append(runtimeArgs, "--cpu-limit", cpuLimit)
	}
	if memLimit != "" {
		runtimeArgs = append(runtimeArgs, "--mem-limit", memLimit)
	}
	runtimeArgs = append(runtimeArgs, rootfs)
	runtimeArgs = append(runtimeArgs, cmdSlice...)

	bin := runtimeBin()
	cmd := exec.Command(bin, runtimeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cspip: pipe error: %v\n", err)
		return 1
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cspip: could not start runtime: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: ensure 'cspip-runtime' is installed in the same directory as 'cspip'\n")
		return 1
	}

	// containerInfo holds the ID and host PID extracted from the runtime's
	// first output line: "Container <id> started (PID <pid>)".
	type containerInfo struct {
		id  string
		pid int
	}
	startCh := make(chan containerInfo, 1)

	// goroutineStartLineFormat is the exact message printed by the C runtime
	// (container.c, container_run) when the container process has been started.
	// Any change to that message must be reflected here.
	const goroutineStartLineFormat = "Container %s started (PID %d)"
	go func() {
		reader := bufio.NewReader(stdoutPipe)

		// Read the first line — this contains "Container <id> started (PID <pid>)".
		firstLine, err := reader.ReadString('\n')
		firstLine = strings.TrimRight(firstLine, "\r\n")
		if firstLine != "" {
			fmt.Println(firstLine) // forward to our stdout
		}
		if err != nil && err != io.EOF {
			startCh <- containerInfo{}
		} else {
			var id string
			var pid int
			if n, _ := fmt.Sscanf(firstLine, goroutineStartLineFormat, &id, &pid); n == 2 && pid > 0 {
				startCh <- containerInfo{id: id, pid: pid}
			} else {
				startCh <- containerInfo{}
			}
		}

		// Forward the remainder of stdout transparently.
		if _, err := io.Copy(os.Stdout, reader); err != nil {
			fmt.Fprintf(os.Stderr, "cspip: stdout forwarding error: %v\n", err)
		}
	}()

	// Wait up to 15 s for the container to start.
	var info containerInfo
	select {
	case info = <-startCh:
	case <-time.After(15 * time.Second):
		fmt.Fprintf(os.Stderr, "warn: timed out waiting for container start output\n")
	}

	// Start the profiler in the background if we have a valid host PID.
	var profilerDone chan []profiler.ProfileSnapshot
	if info.pid > 0 {
		profilerDone = make(chan []profiler.ProfileSnapshot, 1)
		go func(pid int) {
			profilerDone <- profiler.StartSampler(pid)
		}(info.pid)
	}

	// Wait for the runtime process to finish.
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	duration := time.Since(startTime)

	// Collect profiler samples (give the sampler goroutine a brief moment to
	// notice that the process has exited).
	var series []profiler.ProfileSnapshot
	if profilerDone != nil {
		select {
		case s := <-profilerDone:
			series = s
		case <-time.After(3 * time.Second):
			fmt.Fprintf(os.Stderr, "warn: profiler did not finish in time; report will have partial data\n")
		}
	}

	// Persist the run record for later retrieval by `cspip report`.
	if info.id != "" {
		rec := store.RunRecord{
			ContainerID:   info.id,
			Command:       cmdSlice,
			DurationSecs:  duration.Seconds(),
			ExitCode:      exitCode,
			CPULimit:      cpuLimit,
			MemLimit:      memLimit,
			MemLimitBytes: parseMemLimit(memLimit),
			Series:        series,
		}
		if err := store.Save(rec); err != nil {
			fmt.Fprintf(os.Stderr, "warn: could not save run record: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Run record saved. Generate report with: cspip report %s\n", info.id)
		}
	}

	return exitCode
}

// ------------------------------------------------------------------ report command

func cmdReport(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "error: report requires <container-id>\n")
		usage()
		return 1
	}

	containerID := args[0]
	format := "text"
	outputFile := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --format requires an argument\n")
				return 1
			}
			i++
			format = args[i]
		case "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --output requires an argument\n")
				return 1
			}
			i++
			outputFile = args[i]
		default:
			fmt.Fprintf(os.Stderr, "error: unknown option %q\n", args[i])
			return 1
		}
	}

	// Load persisted run record.
	rec, err := store.Load(containerID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Build analysis results.
	wt, conf := analyzer.Classify(rec.Series, rec.MemLimitBytes)
	patterns := analyzer.AnalyzePatterns(rec.Series)
	bottlenecks := analyzer.DetectBottlenecks(rec.Series, rec.ContainerID)

	analysisResult := analyzer.AnalysisResult{
		Classification: analyzer.Classification{
			Type:       wt,
			Confidence: conf,
		},
		Bottlenecks: bottlenecks,
		Patterns:    patterns,
	}

	// Build report.
	alerts := reporter.GenerateAlerts(rec.Series, analysisResult, rec.MemLimitBytes)
	suggestions := reporter.GenerateSuggestions(alerts, reporter.Classification{
		Type:       string(wt),
		Confidence: string(conf),
	})
	metrics := reporter.BuildSummary(rec.Series)

	rpt := reporter.Report{
		ContainerID:  rec.ContainerID,
		Command:      rec.Command,
		DurationSecs: rec.DurationSecs,
		ExitCode:     rec.ExitCode,
		Metrics:      metrics,
		Classification: reporter.Classification{
			Type:       string(analysisResult.Classification.Type),
			Confidence: string(analysisResult.Classification.Confidence),
			Score:      analysisResult.Classification.Score,
			Reason:     analysisResult.Classification.Reason,
		},
		Bottlenecks: alerts,
		Suggestions: suggestions,
	}

	// Choose output destination.
	out := os.Stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot create output file: %v\n", err)
			return 1
		}
		defer f.Close()
		out = f
	}

	switch strings.ToLower(format) {
	case "json":
		if err := reporter.RenderJSON(rpt, out); err != nil {
			fmt.Fprintf(os.Stderr, "error rendering JSON: %v\n", err)
			return 1
		}
	case "text", "":
		reporter.RenderText(rpt, out)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q (use 'text' or 'json')\n", format)
		return 1
	}

	if outputFile != "" {
		fmt.Fprintf(os.Stderr, "Report saved to %s\n", outputFile)
	}
	return 0
}

// ------------------------------------------------------------------ main

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	command := os.Args[1]
	subArgs := os.Args[1:] // include command name for delegation

	switch command {
	case "run":
		os.Exit(cmdRun(subArgs))

	case "report":
		os.Exit(cmdReport(os.Args[2:]))

	case "--help", "-h":
		usage()
		os.Exit(0)

	default:
		// Delegate container lifecycle commands to the C runtime binary:
		// exec, inspect, ps, stop, kill, rm.
		os.Exit(delegateToRuntime(subArgs))
	}
}
