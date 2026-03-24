package profiler

import (
    "bufio"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

func GetSyscallStats(pid int) (map[string]uint64, error) {
    outputFile := "/tmp/strace_out"

    cmd := exec.Command(
        "strace",
        "-c",
        "-p", strconv.Itoa(pid),
        "-e", "trace=all",
        "-f",
        "-o", outputFile,
    )

    err := cmd.Start()
    if err != nil {
        return nil, err
    }

    // sample for 2 seconds
    time.Sleep(2 * time.Second)

    cmd.Process.Kill()
    cmd.Wait()

    file, err := os.Open(outputFile)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    stats := make(map[string]uint64)

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "-") {
            continue
        }

        fields := strings.Fields(line)

        if len(fields) < 5 {
            continue
        }

        calls, err := strconv.ParseUint(fields[3], 10, 64)
        if err != nil {
            continue
        }

        syscallName := fields[4]

        stats[syscallName] = calls
    }

    return stats, nil
}