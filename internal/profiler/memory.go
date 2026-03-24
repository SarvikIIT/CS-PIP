package profiler

import (
    "fmt"
    "os"
    "strconv"
    "strings"
)

func readProcStatus(pid int) (string, error) {
    path := fmt.Sprintf("/proc/%d/status", pid)

    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }

    return string(data), nil
}
func parseVmRSS(data string) (uint64, error) {
    lines := strings.Split(data, "\n")

    for _, line := range lines {
        if strings.HasPrefix(line, "VmRSS:") {
            fields := strings.Fields(line)

            // fields = ["VmRSS:", "128456", "kB"]
            valueKB, err := strconv.ParseUint(fields[1], 10, 64)
            if err != nil {
                return 0, err
            }

            return valueKB * 1024, nil // convert to bytes
        }
    }

    return 0, fmt.Errorf("VmRSS not found")
}
func GetMemoryUsage(pid int) (uint64, error) {
    data, err := readProcStatus(pid)
    if err != nil {
        return 0, err
    }

    return parseVmRSS(data)
}
func readSmapsRollup(pid int) (uint64, uint64, error) {
    path := fmt.Sprintf("/proc/%d/smaps_rollup", pid)

    data, err := os.ReadFile(path)
    if err != nil {
        return 0, 0, err
    }

    var anon, file uint64

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.HasPrefix(line, "Anonymous:") {
            fields := strings.Fields(line)
            val, _ := strconv.ParseUint(fields[1], 10, 64)
            anon = val * 1024
        }

        if strings.HasPrefix(line, "File:") {
            fields := strings.Fields(line)
            val, _ := strconv.ParseUint(fields[1], 10, 64)
            file = val * 1024
        }
    }

    return anon, file, nil
}