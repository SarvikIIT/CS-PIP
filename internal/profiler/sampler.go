package profiler

import (
	"fmt"
	"os"
	"time"
)

// StartSampler polls cgroup v2 aggregate stats until the container's init
// process (pid) exits.  containerID is used to locate the cgroup files at
// /sys/fs/cgroup/cspip/<containerID>/.
//
// Using cgroup aggregates instead of single-PID /proc reads is critical:
// stress workloads spawn child processes that do the actual CPU/memory/I/O
// work; reading only the shell's /proc entry would always return 0.
func StartSampler(pid int, containerID string) []ProfileSnapshot {
	series := make([]ProfileSnapshot, 0, 1000)

	cpuTracker, err := newCgroupCPUTracker(containerID)
	if err != nil {
		return nil
	}

	// Allow the CPU tracker to establish a baseline before the first delta.
	time.Sleep(100 * time.Millisecond)

	for {
		// Stop when the container's init process exits.
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); os.IsNotExist(err) {
			break
		}

		// CPU — cgroup aggregate (covers all child processes).
		cpuPercent, err := cpuTracker.cpuPercent(containerID)
		if err != nil {
			cpuPercent = 0
		}

		// Memory — cgroup aggregate.
		memRSS, err := cgroupMemory(containerID)
		if err != nil {
			memRSS = 0
		}

		// Anonymous / file-backed split from the init process (best-effort).
		anon, file, err := readSmapsRollup(pid)
		if err != nil {
			anon, file = 0, 0
		}

		// Page faults — sum across all live cgroup processes.
		minflt, majflt := cgroupPageFaults(containerID)

		// Context switches — sum across all live cgroup processes.
		vol, invol := cgroupCtxSwitches(containerID)

		// I/O — cgroup io.stat if available, else sum per-PID /proc/io.
		readBytes, writeBytes, _ := cgroupIO(containerID)

		series = append(series, ProfileSnapshot{
			Timestamp:      time.Now(),
			CPUPercent:     cpuPercent,
			MemRSSBytes:    memRSS,
			AnonymousBytes: anon,
			FileBytes:      file,
			MinorFaults:    minflt,
			MajorFaults:    majflt,
			VolCtxSwitches: vol,
			InvCtxSwitches: invol,
			IOReadBytes:    readBytes,
			IOWriteBytes:   writeBytes,
		})

		time.Sleep(100 * time.Millisecond)
	}

	return series
}
