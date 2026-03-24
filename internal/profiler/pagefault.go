package profiler

import (
    "fmt"
    "strconv"
)

func GetPageFaults(pid int) (uint64, uint64, error) {
    fields, err := readProcStat(pid)
    if err != nil {
        return 0, 0, err
    }

    if len(fields) < 12 {
        return 0, 0, fmt.Errorf("unexpected /proc/stat format: only %d fields", len(fields))
    }

    minflt, err := strconv.ParseUint(fields[9], 10, 64)
    if err != nil {
        return 0, 0, err
    }

    majflt, err := strconv.ParseUint(fields[11], 10, 64)
    if err != nil {
        return 0, 0, err
    }

    return minflt, majflt, nil
}