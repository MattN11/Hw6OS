# RAID Simulation in Go

A complete implementation of RAID levels 0, 1, 4, and 5 in Go, with benchmarking tools to see performance across different RAID configurations.

## Project Structure

- `raid.go` - Core RAID implementation with all RAID levels (0, 1, 4, 5)
- `main.go` - Benchmark tool to measure and compare performance
- `go.mod` - Go module definition
- `README.md` - This file

## Features

### RAID Levels Implemented

#### RAID 0 (Striping)
- **Description**: Data is striped across all disks for maximum throughput
- **Capacity**: 100% (in this implementation: 5 disks)
- **Redundancy**: None
- **Expected Performance**: Fastest write and read operations
- **Trade-off**: Any single disk failure results in total data loss

#### RAID 1 (Mirroring)  
- **Description**: Data is mirrored across two disks per block
- **Capacity**: 50% (in this implementation: 2.5 disks effective)
- **Redundancy**: Full (one disk failure is recoverable)
- **Expected Performance**: Good reads (limited by single disk), slower writes (dual writes)
- **Trade-off**: Significant storage overhead due to mirroring

#### RAID 4 (Block-level Striping with Dedicated Parity)
- **Description**: Data striped across data disks, parity stored on dedicated disk
- **Capacity**: 80% (in this implementation: 4 data disks, 1 parity)
- **Redundancy**: One disk failure is recoverable
- **Expected Performance**: Good reads, slower writes due to parity disk bottleneck
- **Trade-off**: Parity disk becomes a write bottleneck

#### RAID 5 (Block-level Striping with Distributed Parity)
- **Description**: Data and parity distributed across all disks
- **Capacity**: 80% (same as RAID 4)
- **Redundancy**: One disk failure is recoverable
- **Expected Performance**: Better writes than RAID 4 (no parity bottleneck), comparable reads
- **Trade-off**: More complex parity calculation and distribution

### Implementation Details

#### Disk Simulation
- Uses 5 regular files (`disk0.dat` to `disk4.dat`) to represent physical disks
- Block size: 4KB (configurable)
- Files are flushed using `fsync` to simulate real disk write delays
- Read/write operations use `ReadAt`/`WriteAt` for random block access

#### Data Layout
- **RAID 0**: Block N goes to disk (N % 5), physical block (N / 5)
- **RAID 1**: Uses mirror pairs (disks 0-1, 2-3) for redundancy
- **RAID 4**: Data in disks 0-3, parity in disk 4, distributed by stripe number
- **RAID 5**: Data and parity distributed across all disks with rotating parity pattern

#### Parity Calculation
- Uses XOR operation for RAID 4 and RAID 5 parity
- Parity is recalculated and rewritten on every write operation

## Building and Running

### Build
```bash
cd raid-simulation
go build -o raid-sim
```

### Run Benchmark
```bash
./raid-sim
```

Or directly with go run:
```bash
go run . main.go raid.go
```

### Configuration

In `main.go`, you can adjust:
- `numBlocks`: Number of blocks to test (default: 24576 ≈ 96MB with 4KB blocks)
- `BlockSize`: Block size in bytes (currently 4KB in `raid.go`)
- `NumDisks`: Number of physical disks (currently 5)

## Benchmark Output

The benchmark tool generates:
1. **Progress indicators** during write and read operations
2. **Summary table** comparing throughput, capacity, and total time for all RAID levels
3. **Detailed metrics** including:
   - Write/read throughput (MB/s)
   - Average time per block
   - Total bytes processed
   - Effective storage capacity
4. **Performance analysis** with comparison to textbook expectations

### Key Insights

1. **RAID 0** dominates in throughput but offers no fault tolerance
2. **RAID 1** provides excellent read performance with full redundancy but uses 50% of capacity for mirroring
3. **RAID 4** suffers from the "parity disk bottleneck" during writes
4. **RAID 5** distributes parity to avoid bottlenecks, offering better write performance than RAID 4

### Potential Variations from Textbook

Real-world performance may vary from textbook predictions due to:
- **File I/O overhead**: Using files instead of actual disks adds filesystem overhead
- **Synchronous writes**: `fsync` calls may not fully represent hardware behavior
- **Buffer effects**: OS caching can amplify or reduce differences
- **Block size effects**: Larger blocks (4KB) may behave differently than 512B predictions
- **Sequential vs Random**: Benchmark uses sequential access; random access patterns would show different results

To get results closer to textbook expectations:
- Increase `numBlocks` for larger dataset (reduces relative overhead)
- Use unbuffered I/O if testing on Linux with O_DIRECT
- Run benchmarks multiple times and average results

## Code Structure

### RAID Interface
All RAID levels implement this interface:
```go
type RAID interface {
    Write(blockNum int, data []byte) error
    Read(blockNum int) ([]byte, error)
    GetCapacity() int64
    Cleanup() error
    GetName() string
}
```

### DiskManager
Handles low-level disk operations:
- `WriteToDisk(diskNum, blockNum, data)`
- `ReadFromDisk(diskNum, blockNum)`
- File management and fsync operations

## Limitations and Notes

1. **No error recovery**: Doesn't simulate disk failures or recovery procedures
2. **Fixed block size**: All writes must be exact block size
3. **Sequential block numbering**: Assumes logical block numbers are sequential
4. **No caching**: Simulates all operations; no caching layer
5. **File format**: Disk files grow dynamically; not pre-allocated

## Future Enhancements

- Add disk failure simulation and recovery
- Support variable-sized write operations
- Implement prefix/suffix blocking for partial blocks
- Add random I/O benchmark workload
- Parallel read/write operations
- Memory-based caching simulation
- Real hardware comparison mode

## References

- RAID concepts from "Modern Operating Systems" and similar textbooks
- XOR parity calculation for RAID 4/5
- Go standard library: os package for file I/O, time for benchmarking


