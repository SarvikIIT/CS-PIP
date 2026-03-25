package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SarvikIIT/CS-PIP/internal/profiler"
)

// DetectBottlenecks analyzes runtime data and cgroup stats
// to identify system-level bottlenecks.
func DetectBottlenecks(series []profiler.ProfileSnapshot, containerID string) []string {
	var bottlenecks []string

	// CPU throttling
	if throttled, err := checkCPUThrottling(containerID); err == nil && throttled {
		bottlenecks = append(bottlenecks, "CPU Throttling")
	}

	// Memory pressure
	if memPressure, err := checkMemoryPressure(containerID); err == nil && memPressure {
		bottlenecks = append(bottlenecks, "Memory Pressure")
	}

	// I/O saturation
	if detectIOSaturation(series) {
		bottlenecks = append(bottlenecks, "I/O Saturation")
	}

	return bottlenecks
}

// ---------------- CPU THROTTLING ----------------

func checkCPUThrottling(containerID string) (bool, error) {
	path := fmt.Sprintf("/sys/fs/cgroup/cspip/%s/cpu.stat", containerID)

	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "nr_throttled") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return false, fmt.Errorf("invalid cpu.stat format")
			}

			val, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return false, err
			}

			return val > 0, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, fmt.Errorf("nr_throttled not found")
}

// ---------------- MEMORY PRESSURE ----------------

func checkMemoryPressure(containerID string) (bool, error) {
	base := fmt.Sprintf("/sys/fs/cgroup/cspip/%s", containerID)

	currentPath := base + "/memory.current"
	maxPath := base + "/memory.max"

	currentData, err := os.ReadFile(currentPath)
	if err != nil {
		return false, err
	}

	maxData, err := os.ReadFile(maxPath)
	if err != nil {
		return false, err
	}

	current, err := strconv.ParseUint(strings.TrimSpace(string(currentData)), 10, 64)
	if err != nil {
		return false, err
	}

	maxStr := strings.TrimSpace(string(maxData))

	// "max" means unlimited memory
	if maxStr == "max" {
		return false, nil
	}

	max, err := strconv.ParseUint(maxStr, 10, 64)
	if err != nil {
		return false, err
	}

	if max == 0 {
		return false, nil
	}

	return float64(current) > 0.9*float64(max), nil
}

// ---------------- IO SATURATION ----------------

func detectIOSaturation(series []profiler.ProfileSnapshot) bool {
	n := len(series)
	if n < 5 {
		return false
	}

	var prevRate float64
	var plateauCount int
	var validSamples int

	for i := 1; i < n; i++ {
		prev := series[i-1]
		curr := series[i]

		dt := curr.Timestamp.Sub(prev.Timestamp).Seconds()
		if dt == 0 {
			continue
		}

		validSamples++

		deltaWrite := curr.IOWriteBytes - prev.IOWriteBytes
		rate := float64(deltaWrite) / dt

		// plateau detection: growth stops
		if i > 2 {
			if rate < prevRate*1.05 { // <5% growth
				plateauCount++
			}
		}

		prevRate = rate
	}

	if validSamples == 0 {
		return false
	}

	return plateauCount > validSamples/4
}