#!/bin/sh
# io_stress.sh — I/O-bound workload for Stage 1 / Stage 2 testing.
#
# Generates sequential and random I/O against a temporary file to produce
# measurable /proc/<pid>/io activity (read_bytes, write_bytes).
#
# Usage: io_stress.sh [file_size_mb] [iterations] [tmp_dir]
#
# Example: ./testdata/workloads/io_stress.sh 32 5 /tmp

FILE_MB=${1:-32}
ITERS=${2:-3}
TMPDIR_BASE=${3:-/tmp}

TMPFILE=$(mktemp "${TMPDIR_BASE}/io_stress_XXXXXX")

echo "I/O stress: ${FILE_MB} MB file, ${ITERS} read/write iterations"

# --- Sequential writes ---
i=0
while [ "$i" -lt "$ITERS" ]; do
    dd if=/dev/zero of="$TMPFILE" bs=1048576 count="$FILE_MB" \
        conv=fsync 2>/dev/null
    echo "  write pass $(( i + 1 ))/${ITERS} done"
    i=$(( i + 1 ))
done

# --- Sequential reads ---
i=0
while [ "$i" -lt "$ITERS" ]; do
    dd if="$TMPFILE" of=/dev/null bs=1048576 2>/dev/null
    echo "  read  pass $(( i + 1 ))/${ITERS} done"
    i=$(( i + 1 ))
done

rm -f "$TMPFILE"
echo "I/O stress done."
