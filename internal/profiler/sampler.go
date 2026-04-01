package profiler

import (
    "context"
    "syscall"
    "time"
)

// StartSampler samples the resource usage of the process identified by pid
// until it exits or ctx is cancelled.  The caller should cancel ctx once the
// monitored process has been waited for to guarantee prompt goroutine exit.
func StartSampler(ctx context.Context, pid int) []ProfileSnapshot {
    series := make([]ProfileSnapshot, 0, 1000)

    cpuTracker, err := NewCPUTracker(pid)
    if err != nil {
        return nil
    }

    // allow CPU tracker to stabilize
    select {
    case <-ctx.Done():
        return series
    case <-time.After(100 * time.Millisecond):
    }

    for {
        // check if process is alive
        if syscall.Kill(pid, 0) != nil {
            break
        }

        // CPU
        cpuPercent, err := cpuTracker.GetCPUPercent(pid)
        if err != nil {
            cpuPercent = 0
        }

        // Memory
        memRSS, err := GetMemoryUsage(pid)
        if err != nil {
            memRSS = 0
        }

        // smaps (optional)
        anon, file, err := readSmapsRollup(pid)
        if err != nil {
            anon = 0
            file = 0
        }

        // Page faults
        minflt, majflt, err := GetPageFaults(pid)
        if err != nil {
            minflt, majflt = 0, 0
        }

        // Context switches
        vol, invol, err := GetContextSwitches(pid)
        if err != nil {
            vol, invol = 0, 0
        }
        readBytes, writeBytes, err := GetIOStats(pid)
        if err != nil {
            readBytes, writeBytes = 0, 0
        }
        snapshot := ProfileSnapshot{
            Timestamp:          time.Now(),
            CPUPercent:         cpuPercent,
            MemRSSBytes:        memRSS,
            AnonymousBytes:     anon,
            FileBytes:          file,
            MinorFaults:        minflt,
            MajorFaults:        majflt,
            VolCtxSwitches:     vol,
            InvCtxSwitches:     invol,
            IOReadBytes:        readBytes,
            IOWriteBytes:       writeBytes,
        }

        series = append(series, snapshot)

        // context-aware sleep: exit promptly when ctx is cancelled
        select {
        case <-ctx.Done():
            return series
        case <-time.After(100 * time.Millisecond):
        }
    }

    return series
}