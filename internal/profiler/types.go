package profiler

import "time"

// ProfileSnapshot represents one point-in-time measurement
// of a container's resource usage.
type ProfileSnapshot struct {
    Timestamp          time.Time  // when the sample was taken

    // CPU
    CPUPercent         float64    // CPU usage %

    // Memory
    MemRSSBytes        uint64     // Resident memory (RAM)
    AnonymousBytes     uint64     // Heap/stack memory
    FileBytes          uint64     // File-backed memory (libs, mmap)

    // Page Faults
    MinorFaults        uint64     // Cheap faults (memory already in RAM)
    MajorFaults        uint64     // Expensive faults (disk access)

    // Context Switches
    VolCtxSwitches     uint64     // Voluntary (waiting, sleep, I/O)
    InvCtxSwitches     uint64     // Involuntary (preempted by scheduler)
    IOReadBytes  uint64
    IOWriteBytes uint64
}

// MetricSeries is a collection of snapshots over time
type MetricSeries []ProfileSnapshot