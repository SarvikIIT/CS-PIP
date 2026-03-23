#!/bin/sh
# cpu_stress.sh — CPU-bound workload for Stage 1 / Stage 2 testing.
#
# Runs a tight arithmetic loop on all available CPUs for DURATION seconds.
# Usage: cpu_stress.sh [duration_seconds] [num_workers]
#
# Example: ./testdata/workloads/cpu_stress.sh 10 2

DURATION=${1:-10}
WORKERS=${2:-1}

echo "CPU stress: ${WORKERS} worker(s) for ${DURATION}s"

# Launch background workers
i=0
while [ "$i" -lt "$WORKERS" ]; do
    (
        end=$(( $(date +%s) + DURATION ))
        while [ "$(date +%s)" -lt "$end" ]; do
            # Tight arithmetic — keeps the CPU fully busy
            x=1
            j=0
            while [ "$j" -lt 100000 ]; do
                x=$(( x + j ))
                j=$(( j + 1 ))
            done
        done
    ) &
    i=$(( i + 1 ))
done

# Wait for all workers
wait
echo "CPU stress done."
