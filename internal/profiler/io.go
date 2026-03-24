package profiler

import (
    "fmt"
    "os"
    "strconv"
    "strings"
)

func GetIOStats(pid int) (uint64, uint64, error) {
    path := fmt.Sprintf("/proc/%d/io", pid)

    data, err := os.ReadFile(path)
    if err != nil {
        return 0, 0, err
    }

    var readBytes uint64
    var writeBytes uint64

    lines := strings.Split(string(data), "\n")

    for _, line := range lines {
        if strings.HasPrefix(line, "read_bytes:") {
            fields := strings.Fields(line)
            if len(fields) >= 2 {
                val, err := strconv.ParseUint(fields[1], 10, 64)
                if err == nil {
                    readBytes = val
                }
            }
        }

        if strings.HasPrefix(line, "write_bytes:") {
            fields := strings.Fields(line)
            if len(fields) >= 2 {
                val, err := strconv.ParseUint(fields[1], 10, 64)
                if err == nil {
                    writeBytes = val
                }
            }
        }
    }

    return readBytes, writeBytes, nil
}