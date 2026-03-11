# CSPIP: Containerized System Profiling and Performance Intelligence Platform

A lightweight Docker-like container runtime with an integrated profiling engine and behavioral analysis framework — built from the ground up using Linux namespaces, cgroups, and /proc-based instrumentation.

---

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture Overview](#architecture-overview)
- [Folder Structure](#folder-structure)
- [Stage-by-Stage Development Guide](#stage-by-stage-development-guide)
  - [Stage 1: Container Runtime](#stage-1-container-runtime-foundation)
  - [Stage 2: Profiling and Monitoring Layer](#stage-2-profiling-and-monitoring-layer)
  - [Stage 3: Behavioral Analysis Engine](#stage-3-behavioral-analysis-engine)
  - [Stage 4: Reporting System](#stage-4-reporting-system)
  - [Stage 5: Advanced Extensions](#stage-5-advanced-extensions)
- [Build and Run](#build-and-run)
- [Dependencies](#dependencies)
- [Evaluation Criteria](#evaluation-criteria)

---

## Project Overview

CSPIP provides:

1. **Container Runtime** — Isolated process execution using Linux namespaces and cgroups, similar to Docker but built from scratch.
2. **Profiling Engine** — Real-time CPU, memory, I/O, and syscall monitoring using `/proc` and kernel tracing interfaces.
3. **Behavioral Analysis** — Automated workload classification, bottleneck detection, and pattern recognition.
4. **Reporting System** — Structured performance intelligence reports exportable as text or JSON.

> **Platform requirement:** Linux kernel 4.6+ (namespaces + cgroup v2 support).

---

## Architecture Overview

```
┌────────────────────────────────────────────────────────┐
│                      CLI (cspip)                       │
│              run | exec | inspect | report             │
└───────────────────────┬────────────────────────────────┘
                        │
          ┌─────────────▼─────────────┐
          │     Container Runtime     │
          │  Namespaces + cgroups     │
          │  pivot_root / chroot      │
          └─────────────┬─────────────┘
                        │
          ┌─────────────▼─────────────┐
          │     Profiling Engine      │
          │  /proc + perf + strace    │
          └─────────────┬─────────────┘
                        │
          ┌─────────────▼─────────────┐
          │   Behavioral Analyzer     │
          │  Classification + Rules   │
          └─────────────┬─────────────┘
                        │
          ┌─────────────▼─────────────┐
          │     Reporting System      │
          │    Text / JSON export     │
          └───────────────────────────┘
```

---

## Folder Structure

```
CSPIP/
│
├── README.md                          # This file
├── Makefile                           # Build targets for all stages
├── go.mod / go.sum                    # Go module files (if using Go)
│   OR CMakeLists.txt                  # (if using C/C++)
│
├── cmd/
│   └── cspip/
│       └── main.go                    # CLI entry point (run/exec/inspect/report)
│
├── internal/
│   │
│   ├── runtime/                       # Stage 1: Container Runtime
│   │   ├── container.go               # Container lifecycle: create, start, stop
│   │   ├── namespace.go               # PID, Mount, UTS namespace setup
│   │   ├── cgroup.go                  # CPU and memory cgroup management
│   │   ├── rootfs.go                  # pivot_root / chroot filesystem isolation
│   │   ├── network.go                 # (optional) basic network namespace
│   │   └── types.go                   # Container config and state structs
│   │
│   ├── profiler/                      # Stage 2: Profiling and Monitoring
│   │   ├── cpu.go                     # CPU usage sampling from /proc/stat
│   │   ├── memory.go                  # Memory tracking from /proc/meminfo, smaps
│   │   ├── pagefault.go               # Major/minor page fault tracking
│   │   ├── context_switch.go          # Voluntary/involuntary context switches
│   │   ├── syscall.go                 # Syscall frequency via strace or seccomp hooks
│   │   ├── io.go                      # I/O stats from /proc/[pid]/io
│   │   ├── sampler.go                 # Unified periodic sampler loop
│   │   └── types.go                   # ProfileSnapshot, MetricSeries structs
│   │
│   ├── analyzer/                      # Stage 3: Behavioral Analysis Engine
│   │   ├── classifier.go              # CPU-bound / Memory-bound / I/O-bound rules
│   │   ├── bottleneck.go              # Threshold-based bottleneck detection
│   │   ├── pattern.go                 # Time-series pattern recognition
│   │   ├── fingerprint.go             # (Stage 5) Behavioral fingerprinting
│   │   └── types.go                   # AnalysisResult, WorkloadClass structs
│   │
│   ├── reporter/                      # Stage 4: Reporting System
│   │   ├── summary.go                 # Execution summary builder
│   │   ├── text_renderer.go           # Human-readable text report
│   │   ├── json_renderer.go           # Machine-readable JSON export
│   │   ├── alert.go                   # Bottleneck alert generation
│   │   └── types.go                   # Report structs
│   │
│   └── store/                         # Persistent run data (Stage 5 comparison)
│       ├── db.go                      # Simple file-based or SQLite store
│       └── types.go                   # RunRecord struct
│
├── pkg/
│   ├── procfs/                        # Low-level /proc parsing utilities
│   │   ├── stat.go                    # Parse /proc/[pid]/stat
│   │   ├── status.go                  # Parse /proc/[pid]/status
│   │   └── io.go                      # Parse /proc/[pid]/io
│   │
│   └── util/
│       ├── logger.go                  # Structured logging
│       └── time.go                    # Ticker / interval helpers
│
├── rootfs/                            # Minimal root filesystem for containers
│   ├── bin/                           # Busybox or static binaries
│   ├── etc/                           # Minimal /etc (hostname, resolv.conf)
│   ├── proc/                          # Mount point for procfs
│   └── tmp/
│
├── testdata/
│   ├── workloads/
│   │   ├── cpu_stress.sh              # CPU-bound test workload
│   │   ├── mem_stress.sh              # Memory-bound test workload
│   │   └── io_stress.sh               # I/O-bound test workload
│   └── expected/
│       └── sample_report.json         # Reference output for validation
│
├── tests/
│   ├── runtime_test.go                # Container lifecycle unit tests
│   ├── profiler_test.go               # Profiler accuracy tests
│   ├── analyzer_test.go               # Classification correctness tests
│   └── integration_test.go            # End-to-end run-and-profile tests
│
└── docs/
    ├── design.md                      # Detailed design decisions
    ├── cgroup_setup.md                # cgroup v1 vs v2 notes
    └── profiling_internals.md         # /proc fields reference
```

---

## Stage-by-Stage Development Guide

---

### Stage 1: Container Runtime (Foundation)

**Goal:** Build a self-contained process sandbox — a mini Docker — using raw Linux kernel features. No Docker, no containerd, no runc. Just your code and the kernel.

**What you are building:** When you run `cspip run /bin/sh`, the shell should:
- Have its own process ID space (PID 1 inside the container)
- Have its own hostname
- See only the files in `rootfs/`
- Be limited to a set CPU % and memory amount

---

#### Step 1.1 — Understand What Namespaces Do

Linux namespaces are the core of containers. Each namespace type hides a different aspect of the host system from the process:

| Namespace | Flag            | What it isolates                         |
|-----------|-----------------|------------------------------------------|
| PID       | `CLONE_NEWPID`  | Process IDs — container gets its own PID 1 |
| Mount     | `CLONE_NEWNS`   | Filesystem mounts — container has its own mount table |
| UTS       | `CLONE_NEWUTS`  | Hostname and domain name                 |
| Network   | `CLONE_NEWNET`  | Network interfaces and routing (optional) |

You create all these at once when you `clone()` the first process of the container.

**File:** `internal/runtime/namespace.go`

```c
// In C — the core clone call that creates the container process
pid_t pid = clone(
    child_func,          // function that runs inside the container
    child_stack_top,     // stack for the child process
    CLONE_NEWPID |       // new PID namespace
    CLONE_NEWNS  |       // new mount namespace
    CLONE_NEWUTS |       // new hostname namespace
    SIGCHLD,             // signal parent when child exits
    &args
);
```

After `clone()`, inside `child_func`:
1. Call `sethostname("my-container", 12)` — sets the hostname visible only inside the container.
2. Call your `setup_rootfs()` function (Step 1.2).
3. Call `mount("proc", "/proc", "proc", 0, NULL)` — mount a fresh `/proc` so tools like `ps` work correctly inside the container.

> **Why mount `/proc`?** Inside a new PID namespace, the kernel creates a new PID table, but the old `/proc` from the host is still visible unless you remount it. Without this step, `ps` inside the container would show host processes.

---

#### Step 1.2 — Set Up Root Filesystem Isolation

This makes the container see only `rootfs/` as its entire filesystem, hiding all host files.

**File:** `internal/runtime/rootfs.go`

**Prepare the rootfs directory on your host first:**
```bash
# Download a minimal filesystem (busybox gives you sh, ls, ps, etc.)
mkdir -p rootfs/bin rootfs/etc rootfs/proc rootfs/tmp
cp $(which busybox) rootfs/bin/busybox

# Create symlinks so sh, ls, ps all work
cd rootfs/bin && for cmd in sh ls ps cat echo mkdir; do
    ln -s busybox $cmd
done
```

**Two approaches to isolate the filesystem:**

**Option A — `chroot` (simpler, less secure):**
```c
chroot("/path/to/rootfs");   // change root directory
chdir("/");                  // move into the new root
```

**Option B — `pivot_root` (correct approach, like real containers):**
```c
// 1. Bind-mount rootfs onto itself (required for pivot_root)
mount("rootfs", "rootfs", NULL, MS_BIND | MS_REC, NULL);

// 2. Create a directory inside rootfs to park the old root
mkdir("rootfs/.old_root", 0700);

// 3. Swap roots
syscall(SYS_pivot_root, "rootfs", "rootfs/.old_root");

// 4. Move into the new root
chdir("/");

// 5. Unmount the old root so host files are truly hidden
umount2("/.old_root", MNT_DETACH);
rmdir("/.old_root");
```

> **Why prefer `pivot_root`?** `chroot` only changes where `/` points for that process, but the old root is still accessible via certain tricks. `pivot_root` actually swaps the mount point at the kernel level — the host filesystem becomes completely unreachable.

---

#### Step 1.3 — Enforce Resource Limits with cgroups

cgroups (control groups) let the kernel enforce hard limits on CPU and memory for a group of processes.

**File:** `internal/runtime/cgroup.go`

**Concept:** Every container gets its own directory under `/sys/fs/cgroup/cspip/`. Writing values into files in that directory sets limits. Adding a PID to `cgroup.procs` puts the process under those limits.

```bash
# The directory structure the kernel creates automatically:
/sys/fs/cgroup/cspip/
└── <container-id>/
    ├── cgroup.procs       ← write PID here to add process to this cgroup
    ├── cpu.max            ← CPU quota: "50000 100000" means 50% of one core
    ├── memory.max         ← Memory limit in bytes: "268435456" = 256 MB
    └── memory.current     ← (read-only) current memory usage
```

**In code:**
```c
// 1. Create the cgroup directory
char cgroup_path[256];
snprintf(cgroup_path, sizeof(cgroup_path),
         "/sys/fs/cgroup/cspip/%s", container_id);
mkdir(cgroup_path, 0755);

// 2. Set CPU limit to 50% (50ms out of every 100ms period)
write_file("/sys/fs/cgroup/cspip/<id>/cpu.max", "50000 100000");

// 3. Set memory limit to 256 MB
write_file("/sys/fs/cgroup/cspip/<id>/memory.max", "268435456");

// 4. Add the container's PID to this cgroup
write_file("/sys/fs/cgroup/cspip/<id>/cgroup.procs", pid_string);
```

> **cgroup v1 vs v2:** Modern Linux (kernel 5.x+) uses cgroup v2 (unified hierarchy). The file names are different from cgroup v1. Check which version your system uses with: `mount | grep cgroup`. This project targets cgroup v2.

---

#### Step 1.4 — Implement Container Lifecycle

A container goes through distinct states. Track them in a JSON file on disk so the CLI can query them.

**File:** `internal/runtime/container.go`

```
States:
  [created] ──start()──▶ [running] ──stop()──▶ [stopped] ──remove()──▶ (deleted)
                                    ──kill()───▶ [stopped]
```

**State file** saved at `/var/run/cspip/<container-id>/state.json`:
```json
{
  "id": "abc123",
  "status": "running",
  "pid": 4821,
  "command": ["/bin/sh"],
  "created_at": "2025-03-12T10:00:00Z",
  "cpu_limit": "50%",
  "mem_limit": "256m",
  "rootfs": "/path/to/rootfs"
}
```

**Functions to implement:**
- `Create(config)` — set up namespace args and cgroup dirs, write state file with status `created`
- `Start(id)` — call `clone()`, set up namespaces/rootfs/mounts, write PID to cgroup, update state to `running`
- `Stop(id)` — send `SIGTERM`, wait for exit, update state to `stopped`
- `Kill(id)` — send `SIGKILL` immediately
- `Remove(id)` — delete cgroup directory, state file, any temp mounts

---

#### Step 1.5 — Build the CLI

**File:** `cmd/cspip/main.go`

```
cspip run  [--cpu-limit 50%] [--mem-limit 256m] <rootfs-path> <command> [args...]
cspip exec <container-id> <command>        # run a new process inside a running container
cspip inspect <container-id>               # print state.json as formatted output
cspip ps                                   # list all containers and their status
cspip stop <container-id>
cspip rm <container-id>
```

**Verification — how to test Stage 1:**
```bash
# Start a container running a shell
sudo ./cspip run --cpu-limit 50% --mem-limit 256m ./rootfs /bin/sh

# Inside the container, these should work differently from the host:
hostname          # should show "my-container", not the host name
ps aux            # should show only processes inside the container (PID 1 = /bin/sh)
ls /              # should show only rootfs/ contents, not host filesystem
cat /proc/self/cgroup  # should show the cspip cgroup path
```

**Evaluation checkpoint:** Container starts, runs in isolation, enforces resource limits, and exits cleanly.

---

### Stage 2: Profiling and Monitoring Layer

**Goal:** While the container runs, continuously observe and record how it uses CPU, memory, and I/O — without significantly slowing it down.

**What you are building:** A background sampler that reads kernel-provided statistics every 100ms and stores them as a time-series. Think of it like a lightweight `top` that records everything instead of just displaying it.

---

#### Step 2.1 — CPU Usage Profiling

**File:** `internal/profiler/cpu.go`

**Where the data comes from:** `/proc/<pid>/stat`

This file has 52+ fields in one line. The fields you need are:

```
Field 14: utime  — CPU time in user mode (in clock ticks)
Field 15: stime  — CPU time in kernel mode (in clock ticks)
Field 16: cutime — user time of waited-for children
Field 17: cstime — kernel time of waited-for children
```

**How to compute CPU %:**

CPU % cannot be read in a single snapshot — you need two snapshots separated by a known time interval:

```
Sample A at time T1:  total_cpu_ticks_A = utime + stime
Sample B at time T2:  total_cpu_ticks_B = utime + stime

elapsed_ticks = total_cpu_ticks_B - total_cpu_ticks_A
elapsed_time  = T2 - T1  (in seconds)
clock_ticks_per_second = sysconf(_SC_CLK_TCK)  // usually 100

cpu_percent = (elapsed_ticks / clock_ticks_per_second) / elapsed_time * 100
```

> **Tip:** On a multi-core system, CPU% can exceed 100% (e.g., 400% means 4 full cores). Divide by the number of CPUs if you want a per-core percentage.

---

#### Step 2.2 — Memory Profiling

**File:** `internal/profiler/memory.go`

**Where the data comes from:** `/proc/<pid>/status`

This file is a human-readable key-value list. The fields you need:

```
VmRSS:    128456 kB   ← Resident Set Size: RAM currently used by this process
VmPeak:   195320 kB   ← Peak RSS ever reached
VmSwap:       0 kB    ← Memory swapped to disk
VmSize:   512000 kB   ← Virtual memory size (not actual RAM)
```

**Also read** `/proc/<pid>/smaps_rollup` for finer breakdown:
```
Anonymous:   64000 kB  ← heap, stack (process-owned RAM)
File:        32000 kB  ← memory-mapped files, shared libs
```

**How to use it:** Record `VmRSS` every sample interval. A steadily increasing RSS that never drops indicates a memory leak.

---

#### Step 2.3 — Page Faults and Context Switches

**File:** `internal/profiler/pagefault.go`, `internal/profiler/context_switch.go`

**Page Faults** — from `/proc/<pid>/stat`:
```
Field 10: minflt  — minor faults (page was in memory, just not mapped — cheap)
Field 12: majflt  — major faults (page had to be read from disk — expensive!)
```
High `majflt` means the process is accessing more memory than is physically available — a sign of memory pressure.

**Context Switches** — from `/proc/<pid>/status`:
```
voluntary_ctxt_switches:    1204   ← process gave up CPU willingly (e.g., waiting for I/O)
nonvoluntary_ctxt_switches:   87   ← kernel forcibly preempted the process (CPU contention)
```
High `nonvoluntary_ctxt_switches` means the process wants more CPU than the scheduler gives it — sign of CPU contention.

---

#### Step 2.4 — Syscall Frequency Analysis

**File:** `internal/profiler/syscall.go`

System calls are how processes ask the kernel to do things (read files, allocate memory, send network data). The distribution of syscalls reveals what kind of work a process is doing.

**Method 1 — Using `strace` (easier, higher overhead):**
```bash
# Attach to a running process and count syscalls for 10 seconds
strace -c -p <pid> -e trace=all 2>&1
# Output shows: syscall name | count | time spent
```
Run this as a subprocess from your Go/C code and parse the output table.

**Method 2 — Using `perf stat` (lower overhead):**
```bash
perf stat -p <pid> -e syscalls:sys_enter_read,syscalls:sys_enter_write -- sleep 1
```

**What the syscall distribution tells you:**
- Mostly `read`/`write` → I/O-bound
- Mostly `mmap`/`brk`/`mprotect` → memory allocation heavy
- Mostly `futex` → multithreaded with locking contention
- Mostly `nanosleep`/`poll` → idle/waiting process

---

#### Step 2.5 — I/O Statistics

**File:** `internal/profiler/io.go`

**Where the data comes from:** `/proc/<pid>/io`

```
rchar:    4194304    ← bytes read from storage (including cache hits)
wchar:    1048576    ← bytes written
syscr:        512    ← number of read syscalls
syscw:        128    ← number of write syscalls
read_bytes:  65536   ← bytes actually fetched from disk (cache misses only)
write_bytes: 32768   ← bytes actually written to disk
```

> **Key distinction:** `rchar` counts all reads including from page cache. `read_bytes` counts only reads that went to physical disk. High `read_bytes` relative to `rchar` = low cache hit rate = I/O bottleneck.

---

#### Step 2.6 — Unified Sampler Loop

**File:** `internal/profiler/sampler.go`

This ties all the collectors together into a single background loop.

```
Every 100ms:
  1. Read /proc/<pid>/stat    → cpu, pagefaults
  2. Read /proc/<pid>/status  → memory, context switches
  3. Read /proc/<pid>/io      → I/O bytes
  4. Compute deltas from last sample
  5. Append ProfileSnapshot{timestamp, cpu%, rss, majflt, io_bytes, ...} to series[]
  6. If process has exited, stop loop and finalize
```

**Data structure** (`internal/profiler/types.go`):
```go
type ProfileSnapshot struct {
    Timestamp      time.Time
    CPUPercent     float64
    MemRSSBytes    uint64
    MajorFaults    uint64
    MinorFaults    uint64
    VolCtxSwitches uint64
    InvCtxSwitches uint64
    IOReadBytes    uint64
    IOWriteBytes   uint64
}

type MetricSeries []ProfileSnapshot
```

**Verification — how to test Stage 2:**
```bash
# Run a CPU stress test inside a container and compare readings
sudo ./cspip run ./rootfs /bin/stress --cpu 2 --timeout 10 &
# In another terminal, compare cspip's output to standard tools:
top -p <container-pid>      # should show similar CPU %
cat /proc/<pid>/io          # should match cspip's I/O numbers
```

**Evaluation checkpoint:** Metrics match values from `top`, `vmstat`, `iostat` within 5% margin.

---

### Stage 3: Behavioral Analysis Engine

**Goal:** Turn raw numbers into understanding. Given the time-series of metrics from Stage 2, automatically answer: "What kind of workload is this, and where is it struggling?"

**What you are building:** A rule engine that reads `[]ProfileSnapshot` and outputs a structured `AnalysisResult` with workload class, bottlenecks found, and usage patterns.

---

#### Step 3.1 — Workload Classification

**File:** `internal/analyzer/classifier.go`

**Concept:** Compute time-averaged values across the entire run, then compare against thresholds.

```
avg_cpu    = mean(snapshot.CPUPercent    for all snapshots)
avg_rss    = mean(snapshot.MemRSSBytes   for all snapshots)
avg_io     = mean(snapshot.IOReadBytes + snapshot.IOWriteBytes for all snapshots)
avg_majflt = mean(snapshot.MajorFaults   for all snapshots)
```

**Classification rules (apply in order):**

```
IF avg_cpu > 70% AND avg_io < 1 MB/s
    → CPU-bound
    → Reason: process spends most time computing, not waiting for data

ELSE IF avg_majflt > 100/s OR avg_rss > 80% of mem_limit
    → Memory-bound
    → Reason: process causes frequent disk reads due to insufficient RAM

ELSE IF avg_io > 10 MB/s OR avg_cpu < 30% (process sleeps waiting for I/O)
    → I/O-bound
    → Reason: process spends most time waiting for reads/writes to complete

ELSE IF avg_cpu > 50% AND avg_io > 5 MB/s
    → Mixed (CPU + I/O)

ELSE
    → Undetermined / Idle
```

**Confidence scoring:** Count how many samples exceeded the threshold (not just the average). If 90%+ of samples triggered a rule, confidence = HIGH. If 60–90%, confidence = MEDIUM.

---

#### Step 3.2 — Bottleneck Detection

**File:** `internal/analyzer/bottleneck.go`

Bottlenecks are specific, actionable problems — not just classifications.

**CPU Throttling Detection:**
```bash
# Read from the cgroup stats file
cat /sys/fs/cgroup/cspip/<id>/cpu.stat

# Look for:
nr_throttled    42      ← number of times the process was throttled
throttled_usec  83200   ← total microseconds spent being throttled
```
If `nr_throttled > 0` → flag **CPU Throttling** bottleneck.

**Memory Pressure Detection:**
```bash
cat /sys/fs/cgroup/cspip/<id>/memory.pressure
# Or compare:
cat /sys/fs/cgroup/cspip/<id>/memory.current   ← actual usage in bytes
cat /sys/fs/cgroup/cspip/<id>/memory.max       ← limit in bytes
```
If `memory.current > 90% of memory.max` → flag **Memory Pressure** bottleneck.

**I/O Saturation Detection:**
Watch the `IOWriteBytes` time-series. If write throughput was growing then suddenly plateaued while the process is still running actively → the I/O subsystem is saturated (disk write queue is full).

---

#### Step 3.3 — Pattern Analysis

**File:** `internal/analyzer/pattern.go`

Look at the shape of the time-series, not just averages.

**Memory Leak Detection:**
```
Take snapshots of VmRSS at t=0, t=25%, t=50%, t=75%, t=100% of run duration.
If each value is consistently larger than the previous → monotonically increasing RSS
→ Flag: Potential memory leak (RSS grew X MB over Y seconds without dropping)
```

**Bursty vs. Steady CPU:**
```
Compute standard deviation of CPUPercent across all snapshots.
If stddev > 30% of mean → Bursty CPU usage (e.g., batch jobs, GC pauses)
If stddev < 10% of mean → Steady CPU usage (e.g., compute loops)
```

**Periodic I/O Spikes:**
```
Scan IOWriteBytes time-series for values > 3x the median.
Count how many such spikes occurred and their spacing.
If spikes are regularly spaced → periodic I/O (e.g., checkpointing, logging)
```

**Evaluation checkpoint:** Classification is correct on all three test workloads:
- `testdata/workloads/cpu_stress.sh` → must classify as CPU-bound
- `testdata/workloads/mem_stress.sh` → must classify as Memory-bound
- `testdata/workloads/io_stress.sh`  → must classify as I/O-bound

---

### Stage 4: Reporting System

**Goal:** Transform the `AnalysisResult` into a clear, structured report that a developer can act on immediately.

---

#### Step 4.1 — Aggregate Summary Statistics

**File:** `internal/reporter/summary.go`

For each metric, compute these aggregate values across all `ProfileSnapshot` entries:

| Statistic | How to compute                                     |
|-----------|----------------------------------------------------|
| Min       | Smallest value seen across all snapshots           |
| Max       | Largest value seen                                 |
| Average   | Sum of all values ÷ number of snapshots            |
| P95       | Sort all values; take the value at the 95th percentile position |

P95 (95th percentile) is more useful than max because max is often a single spike. P95 tells you what the "worst normal" performance looks like.

---

#### Step 4.2 — Text Report Format

**File:** `internal/reporter/text_renderer.go`

```
=== CSPIP Execution Report ===
Container ID  : abc123
Command       : /bin/stress --cpu 4
Duration      : 12.4s
Exit Code     : 0

--- Resource Usage ---
Metric                  Min       Avg       P95       Max
CPU %                   72.1%     87.3%     97.8%     99.1%
Memory RSS (MB)         110       128       188       192
Major Page Faults/s       0         0         2         8
Context Switches/s      120       210       380       450
I/O Read (MB/s)           0       0.1       0.4       1.2
I/O Write (MB/s)          0       0.0       0.1       0.3

--- Workload Classification ---
Type        : CPU-bound
Confidence  : HIGH (93% of samples exceeded CPU threshold)
Reasoning   : Average CPU 87.3%, average I/O 0.1 MB/s

--- Bottlenecks Detected ---
[CRITICAL] CPU throttling: process was throttled 42 times (83ms total)
[WARNING]  Bursty CPU pattern detected (stddev = 11.2% of mean)

--- Optimization Suggestions ---
[*] Increase the CPU quota (--cpu-limit) or reduce the number of worker threads.
[*] Consider spreading CPU-intensive work across time to reduce peak usage.

=== End of Report ===
```

---

#### Step 4.3 — JSON Report Format

**File:** `internal/reporter/json_renderer.go`

```json
{
  "container_id": "abc123",
  "command": ["/bin/stress", "--cpu", "4"],
  "duration_seconds": 12.4,
  "exit_code": 0,
  "metrics": {
    "cpu_percent":    { "min": 72.1, "avg": 87.3, "p95": 97.8, "max": 99.1 },
    "mem_rss_mb":     { "min": 110,  "avg": 128,  "p95": 188,  "max": 192  },
    "io_read_mbps":   { "min": 0.0,  "avg": 0.1,  "p95": 0.4,  "max": 1.2  },
    "io_write_mbps":  { "min": 0.0,  "avg": 0.0,  "p95": 0.1,  "max": 0.3  },
    "major_faults_ps":{ "min": 0,    "avg": 0,    "p95": 2,    "max": 8    }
  },
  "classification": {
    "type": "CPU-bound",
    "confidence": "HIGH",
    "score": 0.93
  },
  "bottlenecks": [
    { "severity": "CRITICAL", "type": "cpu_throttling",
      "detail": "throttled 42 times, 83ms total" },
    { "severity": "WARNING",  "type": "bursty_cpu",
      "detail": "stddev 11.2% of mean" }
  ],
  "suggestions": [
    "Increase CPU quota or reduce thread count.",
    "Spread CPU work across time to reduce peak usage."
  ]
}
```

---

#### Step 4.4 — Alert System

**File:** `internal/reporter/alert.go`

Alerts fire when metrics cross thresholds that you define in a config or as constants:

| Alert                | Trigger Condition                               | Severity |
|----------------------|-------------------------------------------------|----------|
| CPU Throttling       | `nr_throttled > 0` in cgroup cpu.stat           | CRITICAL |
| Memory Near Limit    | `memory.current > 90% of memory.max`            | WARNING  |
| High Major Faults    | `majflt/s > 50`                                 | WARNING  |
| Memory Leak          | RSS increased by > 20% monotonically            | WARNING  |
| I/O Saturation       | Write throughput plateau while process is active| WARNING  |

**CLI integration:**
```bash
cspip report <container-id>                       # text report to stdout
cspip report <container-id> --format json         # JSON to stdout
cspip report <container-id> --output report.json  # save to file
```

**Evaluation checkpoint:** Running `cspip report` after each test workload produces an accurate report matching `testdata/expected/sample_report.json`.

---

### Stage 5: Advanced Extensions

**Goal:** Add intelligence that grows with repeated use — comparing runs, fingerprinting behavior, and automatically adjusting profiling intensity.

---

#### Step 5.1 — Run-to-Run Comparison

**Files:** `internal/store/db.go`, `internal/store/types.go`

**How it works:**
1. At the end of every run, serialize the `AnalysisResult` to disk as a JSON file in `~/.cspip/runs/<run-id>.json`.
2. The `cspip compare` command loads two run files and computes deltas.

```bash
cspip compare <run-id-1> <run-id-2>

# Output:
Metric              Run A       Run B       Delta
CPU avg             87.3%       61.2%       -26.1%  ✓ improved
Memory peak         192 MB      210 MB      +18 MB  ✗ regressed
I/O Write           0.1 MB/s    4.2 MB/s    +4.1 MB/s (new I/O pattern)
Classification      CPU-bound   Mixed       changed
```

This is useful for benchmarking code changes — run the same workload before and after a change and see if it got better or worse.

---

#### Step 5.2 — Behavioral Fingerprinting

**File:** `internal/analyzer/fingerprint.go`

**Concept:** Encode each run as a fixed-length vector of normalized metrics, then compare vectors across runs to detect regressions.

```
Fingerprint vector = [
  avg_cpu_normalized,       // 0.0 to 1.0
  avg_mem_normalized,       // relative to cgroup limit
  avg_io_normalized,        // relative to disk bandwidth
  syscall_read_fraction,    // fraction of syscalls that are read()
  syscall_write_fraction,
  context_switch_rate,
  major_fault_rate
]
```

**Distance metric:** Use Euclidean distance or cosine similarity between two fingerprint vectors.
- Distance near 0 → runs are behaviorally identical
- Distance > 0.3 → significant behavioral change detected

**Use case:** Automatically flag if a new version of your application has a meaningfully different resource profile from the previous version.

---

#### Step 5.3 — Adaptive Profiling

**Concept:** Sampling every 100ms all the time wastes CPU when nothing is changing. Sampling too infrequently misses short bursts.

**Algorithm:**

```
Start: sample_interval = 1000ms (coarse)

Every sample:
  Compute variance of last 5 CPU readings.
  IF variance > threshold:
    sample_interval = max(50ms, sample_interval / 2)   ← zoom in
    log("High variance detected, increasing sample rate")
  ELSE IF variance < threshold/4:
    sample_interval = min(1000ms, sample_interval * 2)  ← zoom out
    log("Low variance, reducing sample rate")
```

This gives fine-grained data during bursty or unusual behavior, and low overhead during steady-state execution.

---

## Build and Run

```bash
# Prerequisites
sudo apt-get install golang busybox-static strace linux-perf

# Initialize Go module
go mod init github.com/yourname/cspip

# Build the binary
make build
# OR: go build -o cspip ./cmd/cspip/

# Run a container with limits
sudo ./cspip run --cpu-limit 50% --mem-limit 256m ./rootfs /bin/sh

# Run all test workloads and print reports
make test-workloads

# Run unit tests
make test

# Generate a report
sudo ./cspip report <container-id>
sudo ./cspip report <container-id> --format json --output report.json

# Compare two runs
./cspip compare <run-id-1> <run-id-2>
```

---

## Dependencies

| Dependency        | Purpose                                          | Required? |
|-------------------|--------------------------------------------------|-----------|
| Linux kernel 4.6+ | Namespace + cgroup v2 support                    | Yes       |
| Go 1.21+          | Primary implementation language                  | Yes       |
| `busybox` static  | Minimal rootfs binaries (`sh`, `ls`, `ps`, etc.) | Yes       |
| `strace`          | Syscall frequency counting (Stage 2)             | Optional  |
| `perf`            | Low-overhead CPU event counting                  | Optional  |

> All kernel interfaces (`/proc`, `/sys/fs/cgroup`, `clone(2)`, `pivot_root(2)`) are used directly — no Docker, containerd, or runc dependencies.

---

## Evaluation Criteria

| Stage | What is Tested                                                        | Pass Condition                                   |
|-------|-----------------------------------------------------------------------|--------------------------------------------------|
| 1     | Namespace isolation, rootfs isolation, cgroup enforcement, lifecycle  | `ps`/`hostname`/`ls` behave correctly inside container; limits are enforced |
| 2     | Metric accuracy and profiler overhead                                 | CPU/mem/IO readings within 5% of `top`/`iostat`; profiler uses < 2% CPU |
| 3     | Workload classification correctness                                   | Correct class for all 3 test workloads with HIGH confidence |
| 4     | Report clarity, alert accuracy, JSON schema validity                  | Reports match expected output; JSON passes schema validation |
| 5     | Comparison accuracy, fingerprinting usefulness, adaptive behavior     | Regressions detected; adaptive sampling reduces overhead |
