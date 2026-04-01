package profiler

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const cgroupRoot = "/sys/fs/cgroup/cspip"

func cgroupFile(containerID, name string) string {
	return fmt.Sprintf("%s/%s/%s", cgroupRoot, containerID, name)
}

// cgroupMemory returns the current RSS for all cgroup processes in bytes.
// Reads memory.current which is the authoritative cgroup v2 aggregate.
func cgroupMemory(containerID string) (uint64, error) {
	data, err := os.ReadFile(cgroupFile(containerID, "memory.current"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

// cgroupUsageUsec returns cumulative CPU time in microseconds for the cgroup.
func cgroupUsageUsec(containerID string) (uint64, error) {
	f, err := os.Open(cgroupFile(containerID, "cpu.stat"))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) == 2 && parts[0] == "usage_usec" {
			return strconv.ParseUint(parts[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("cgroup cpu.stat: usage_usec not found")
}

// cgroupIO returns cumulative bytes read and written across all block devices.
// Reads io.stat which requires the io controller to be enabled.
// Falls back to summing /proc/<pid>/io across all cgroup PIDs if unavailable.
func cgroupIO(containerID string) (readBytes, writeBytes uint64, err error) {
	data, err := os.ReadFile(cgroupFile(containerID, "io.stat"))
	if err == nil {
		// Parse "MAJ:MIN rbytes=N wbytes=N rios=N wios=N dbytes=N dios=N"
		for _, line := range strings.Split(string(data), "\n") {
			for _, tok := range strings.Fields(line) {
				if strings.HasPrefix(tok, "rbytes=") {
					if v, e := strconv.ParseUint(tok[7:], 10, 64); e == nil {
						readBytes += v
					}
				}
				if strings.HasPrefix(tok, "wbytes=") {
					if v, e := strconv.ParseUint(tok[7:], 10, 64); e == nil {
						writeBytes += v
					}
				}
			}
		}
		return readBytes, writeBytes, nil
	}
	// Fallback: sum /proc/<pid>/io for all live cgroup processes.
	for _, pid := range cgroupPIDs(containerID) {
		r, w, e := GetIOStats(pid)
		if e == nil {
			readBytes += r
			writeBytes += w
		}
	}
	return readBytes, writeBytes, nil
}

// cgroupPIDs returns the PIDs of all processes currently in the cgroup.
func cgroupPIDs(containerID string) []int {
	data, err := os.ReadFile(cgroupFile(containerID, "cgroup.procs"))
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if pid, e := strconv.Atoi(line); e == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// cgroupPageFaults sums minflt and majflt across all live cgroup processes.
func cgroupPageFaults(containerID string) (minflt, majflt uint64) {
	for _, pid := range cgroupPIDs(containerID) {
		min, maj, err := GetPageFaults(pid)
		if err == nil {
			minflt += min
			majflt += maj
		}
	}
	return
}

// cgroupCtxSwitches sums voluntary and involuntary context switches across
// all live cgroup processes.
func cgroupCtxSwitches(containerID string) (vol, invol uint64) {
	for _, pid := range cgroupPIDs(containerID) {
		v, inv, err := GetContextSwitches(pid)
		if err == nil {
			vol += v
			invol += inv
		}
	}
	return
}

// cgroupCPUTracker computes CPU % from cgroup cpu.stat usage_usec deltas.
// Covers all processes in the cgroup, not just the init process.
type cgroupCPUTracker struct {
	prevUsec uint64
	prevTime time.Time
}

func newCgroupCPUTracker(containerID string) (*cgroupCPUTracker, error) {
	usec, err := cgroupUsageUsec(containerID)
	if err != nil {
		return nil, err
	}
	return &cgroupCPUTracker{prevUsec: usec, prevTime: time.Now()}, nil
}

// cpuPercent returns CPU usage as a percentage of total host CPU capacity
// (100% = all cores fully utilised), consistent with the per-PID tracker.
func (t *cgroupCPUTracker) cpuPercent(containerID string) (float64, error) {
	usec, err := cgroupUsageUsec(containerID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	deltaUsec := usec - t.prevUsec
	deltaSec := now.Sub(t.prevTime).Seconds()
	t.prevUsec = usec
	t.prevTime = now
	if deltaSec <= 0 {
		return 0, nil
	}
	pct := (float64(deltaUsec) / 1e6) / deltaSec * 100.0 / float64(runtime.NumCPU())
	return pct, nil
}
