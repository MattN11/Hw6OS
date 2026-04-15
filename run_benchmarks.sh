#!/bin/bash
# Run RAID benchmarks with different configurations
# This script runs the RAID simulation with various workload sizes

echo "RAID Simulation Benchmark Suite"
echo "================================"

# Build if not already built
if [ ! -f "./raid-sim" ]; then
    echo "Building..."
    go build -o raid-sim
fi

echo ""
echo "Running benchmarks with different workload sizes..."
echo ""

# Very fast mode (4 MB)
echo "=== VERY FAST MODE (4 MB) ==="
./raid-sim -veryfast
echo ""

# Fast mode (24 MB)
echo "=== FAST MODE (24 MB) ==="
./raid-sim -fast
echo ""

# Standard mode (96 MB)
echo "=== STANDARD MODE (96 MB) ==="
./raid-sim
echo ""

# Production mode (384 MB - for larger analysis)
echo "=== PRODUCTION MODE (384 MB) ==="
./raid-sim -blocks 98304
