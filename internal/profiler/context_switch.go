package profiler

import (
    "strconv"
    "strings"
)

func GetContextSwitches(pid int) (uint64, uint64, error) {
    data, err := readProcStatus(pid)
    if err != nil {
        return 0, 0, err
    }

    var vol, invol uint64

    lines := strings.Split(data, "\n")

    for _, line := range lines {
        if strings.HasPrefix(line, "voluntary_ctxt_switches") {
            fields := strings.Fields(line)
            if len(fields) >= 2 {
                val, err := strconv.ParseUint(fields[1], 10, 64)
                if err == nil {
                    vol = val
                }
            }
        }

        if strings.HasPrefix(line, "nonvoluntary_ctxt_switches") {
            fields := strings.Fields(line)
            if len(fields) >= 2 {
                val, err := strconv.ParseUint(fields[1], 10, 64)
                if err == nil {
                    invol = val
                }
            }
        }
    }

    return vol, invol, nil
}