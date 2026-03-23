#!/bin/sh
# mem_stress.sh — Memory-bound workload for Stage 1 / Stage 2 testing.
#
# Allocates growing chunks of memory, touches every page to fault them in,
# then sleeps to let the profiler sample RSS over time.
#
# Usage: mem_stress.sh [target_mb] [hold_seconds]
#
# Example: ./testdata/workloads/mem_stress.sh 128 10

TARGET_MB=${1:-64}
HOLD=${2:-10}

echo "Memory stress: allocating ~${TARGET_MB} MB, holding for ${HOLD}s"

# dd into /dev/null reads from /dev/zero which forces page faults.
# We accumulate data in /tmp so RSS actually grows.
TMPFILE=$(mktemp /tmp/mem_stress_XXXXXX)

# Each dd writes 1 MB; repeat TARGET_MB times.
i=0
while [ "$i" -lt "$TARGET_MB" ]; do
    dd if=/dev/zero bs=1048576 count=1 >> "$TMPFILE" 2>/dev/null
    i=$(( i + 1 ))
done

# Touch the file to ensure pages are faulted in.
wc -c "$TMPFILE" > /dev/null

echo "Allocated. Holding for ${HOLD}s..."
sleep "$HOLD"

rm -f "$TMPFILE"
echo "Memory stress done."
