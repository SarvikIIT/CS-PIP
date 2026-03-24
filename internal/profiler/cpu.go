func readProcStat(pid int) ([]string, error) {
    path := fmt.Sprintf("/proc/%d/stat", pid)

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    fields := strings.Fields(string(data))
    return fields, nil
}

func getCPUTicks(fields []string) (uint64, error) {
    utime, err := strconv.ParseUint(fields[13], 10, 64)
    if err != nil {
        return 0, err
    }

    stime, err := strconv.ParseUint(fields[14], 10, 64)
    if err != nil {
        return 0, err
    }

    return utime + stime, nil
}

type CPUTracker struct {
    prevTicks uint64
    prevTime  time.Time
}

func NewCPUTracker(pid int) (*CPUTracker, error) {
    fields, err := readProcStat(pid)
    if err != nil {
        return nil, err
    }

    ticks, err := getCPUTicks(fields)
    if err != nil {
        return nil, err
    }

    return &CPUTracker{
        prevTicks: ticks,
        prevTime:  time.Now(),
    }, nil
}

func (c *CPUTracker) GetCPUPercent(pid int) (float64, error) {
    fields, err := readProcStat(pid)
    if err != nil {
        return 0, err
    }

    currTicks, err := getCPUTicks(fields)
    if err != nil {
        return 0, err
    }

    currTime := time.Now()

    // compute deltas
    deltaTicks := currTicks - c.prevTicks
    deltaTime := currTime.Sub(c.prevTime).Seconds()

    // update previous values
    c.prevTicks = currTicks
    c.prevTime = currTime

    // clock ticks per second (usually 100)
    clockTicks := float64(100)

    cpuPercent := (float64(deltaTicks) / clockTicks) / deltaTime * 100
    numCPU := runtime.NumCPU()
    cpuPercent = cpuPercent / float64(numCPU)
    return cpuPercent, nil
}