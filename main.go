package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

var (
	numBlocksFlag = flag.Int("blocks", 24576, "Number of blocks to test (default: 24576 ~96MB)")
	fastMode      = flag.Bool("fast", false, "Fast mode: test with fewer blocks (6144 ~24MB)")
	veryFastMode  = flag.Bool("veryfast", false, "Very fast mode: test with minimal blocks (1024 ~4MB)")
)

type BenchmarkResult struct {
	RAIDName             string
	WriteTime            time.Duration
	ReadTime             time.Duration
	TotalTime            time.Duration
	BytesWritten         int64
	BytesRead            int64
	EffectiveCapacity    int64
	WriteThroughputMBps  float64
	ReadThroughputMBps   float64
	AvgWriteTimePerBlock time.Duration
	AvgReadTimePerBlock  time.Duration
}

func runBenchmark(raidSystem RAID, numBlocks int) (*BenchmarkResult, error) {
	result := &BenchmarkResult{
		RAIDName:          raidSystem.GetName(),
		EffectiveCapacity: raidSystem.GetCapacity(),
	}

	// Generate test data
	testData := make([]byte, BlockSize)
	_, _ = rand.Read(testData)

	// ===== WRITE BENCHMARK =====
	fmt.Printf("Running write benchmark for %s with %d blocks...\n", raidSystem.GetName(), numBlocks)
	writeStart := time.Now()

	for block := 0; block < numBlocks; block++ {
		if err := raidSystem.Write(block, testData); err != nil {
			return nil, fmt.Errorf("write error at block %d: %v", block, err)
		}
		if numBlocks > 1000 && (block+1)%(numBlocks/10) == 0 {
			fmt.Printf("  Progress: %d/%d blocks written\n", block+1, numBlocks)
		}
	}

	result.WriteTime = time.Since(writeStart)
	result.BytesWritten = int64(numBlocks) * int64(BlockSize)
	result.WriteThroughputMBps = float64(result.BytesWritten) / (1024 * 1024) / result.WriteTime.Seconds()
	result.AvgWriteTimePerBlock = result.WriteTime / time.Duration(numBlocks)

	// ===== READ BENCHMARK =====
	fmt.Printf("Running read benchmark for %s with %d blocks...\n", raidSystem.GetName(), numBlocks)
	readStart := time.Now()

	for block := 0; block < numBlocks; block++ {
		if _, err := raidSystem.Read(block); err != nil {
			return nil, fmt.Errorf("read error at block %d: %v", block, err)
		}
		if numBlocks > 1000 && (block+1)%(numBlocks/10) == 0 {
			fmt.Printf("  Progress: %d/%d blocks read\n", block+1, numBlocks)
		}
	}

	result.ReadTime = time.Since(readStart)
	result.BytesRead = int64(numBlocks) * int64(BlockSize)

	// Avoid division by zero for very fast reads (filesystem cache)
	if result.ReadTime > 0 {
		result.ReadThroughputMBps = float64(result.BytesRead) / (1024 * 1024) / result.ReadTime.Seconds()
	} else {
		result.ReadThroughputMBps = 0 // Indicate cache hit, too fast to measure
	}

	result.AvgReadTimePerBlock = result.ReadTime / time.Duration(numBlocks)
	result.TotalTime = result.WriteTime + result.ReadTime

	return result, nil
}

func printResults(results []*BenchmarkResult) {
	fmt.Println("\n================================================================================")
	fmt.Println("RAID PERFORMANCE BENCHMARK RESULTS")
	fmt.Println("================================================================================")

	// Summary table
	fmt.Printf("\n%-40s | %15s | %15s | %20s | %20s\n", "RAID Level", "Write (MB/s)", "Read (MB/s)", "Eff. Capacity (MB)", "Total Time (s)")
	fmt.Println("--------------------------------------------------------------------------------")

	for _, r := range results {
		readThroughput := r.ReadThroughputMBps
		readStr := "CACHE"
		if readThroughput > 0 {
			readStr = fmt.Sprintf("%.2f", readThroughput)
		}
		fmt.Printf("%-40s | %15.2f | %15s | %20.2f | %20.3f\n",
			r.RAIDName,
			r.WriteThroughputMBps,
			readStr,
			float64(r.EffectiveCapacity)/(1024*1024),
			r.TotalTime.Seconds())
	}

	// Detailed information
	fmt.Println("\n================================================================================")
	fmt.Println("DETAILED PERFORMANCE METRICS")
	fmt.Println("================================================================================")

	for _, r := range results {
		fmt.Printf("\n%s\n", r.RAIDName)
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("  Write Performance:\n")
		fmt.Printf("    Total Time:        %v\n", r.WriteTime)
		fmt.Printf("    Throughput:        %.2f MB/s\n", r.WriteThroughputMBps)
		fmt.Printf("    Avg per block:     %v\n", r.AvgWriteTimePerBlock)
		fmt.Printf("    Total Bytes:       %.2f MB\n", float64(r.BytesWritten)/(1024*1024))

		fmt.Printf("  Read Performance:\n")
		fmt.Printf("    Total Time:        %v\n", r.ReadTime)
		fmt.Printf("    Throughput:        %.2f MB/s\n", r.ReadThroughputMBps)
		fmt.Printf("    Avg per block:     %v\n", r.AvgReadTimePerBlock)
		fmt.Printf("    Total Bytes:       %.2f MB\n", float64(r.BytesRead)/(1024*1024))

		fmt.Printf("  Storage:\n")
		fmt.Printf("    Effective Capacity: %.2f MB\n", float64(r.EffectiveCapacity)/(1024*1024))
	}

	// Analysis and comparison
	fmt.Println("\n================================================================================")
	fmt.Println("PERFORMANCE ANALYSIS & TEXTBOOK COMPARISON")
	fmt.Println("================================================================================")

	fmt.Println(`
RAID 0 (Striping):
  - Expected: Highest write and read throughput (no parity calculation or mirroring overhead)
  - Trade-off: No redundancy; failure of any disk means data loss
  - Textbook prediction: Should dominate in throughput

RAID 1 (Mirroring):
  - Expected: Read throughput similar to RAID 0, write throughput ~50% due to mirroring both disks
  - Trade-off: 50% effective storage capacity, full redundancy
  - Textbook prediction: Good read performance, reduced write due to dual writes

RAID 4 (Dedicated Parity):
  - Expected: Write slower than RAID 0 due to parity calculation and write to parity disk
  - Trade-off: Near full capacity (80% with 5 disks), redundancy, but parity disk bottleneck
  - Textbook prediction: Parity disk becomes bottleneck for write operations

RAID 5 (Distributed Parity):
  - Expected: Better write performance than RAID 4 due to distributed parity (no bottleneck disk)
  - Trade-off: Near full capacity (80% with 5 disks), good redundancy
  - Textbook prediction: Should outperform RAID 4 in writes, similar to RAID 4 in reads
`)

	// Detailed trend analysis
	if len(results) >= 4 {
		fmt.Println("\nTrend Analysis:")
		fmt.Printf("  RAID 0 write throughput:   %.2f MB/s (baseline: 100%%)\n", results[0].WriteThroughputMBps)

		if len(results) > 1 {
			raid1WriteRatio := (results[1].WriteThroughputMBps / results[0].WriteThroughputMBps) * 100
			fmt.Printf("  RAID 1 write throughput:   %.2f MB/s (%.1f%% of RAID 0)\n",
				results[1].WriteThroughputMBps, raid1WriteRatio)
		}

		if len(results) > 2 {
			raid4WriteRatio := (results[2].WriteThroughputMBps / results[0].WriteThroughputMBps) * 100
			fmt.Printf("  RAID 4 write throughput:   %.2f MB/s (%.1f%% of RAID 0)\n",
				results[2].WriteThroughputMBps, raid4WriteRatio)
		}

		if len(results) > 3 {
			raid5WriteRatio := (results[3].WriteThroughputMBps / results[0].WriteThroughputMBps) * 100
			fmt.Printf("  RAID 5 write throughput:   %.2f MB/s (%.1f%% of RAID 0)\n",
				results[3].WriteThroughputMBps, raid5WriteRatio)

			if results[2].WriteThroughputMBps > 0 {
				raid5to4Ratio := (results[3].WriteThroughputMBps / results[2].WriteThroughputMBps) * 100
				fmt.Printf("  RAID 5 vs RAID 4:          %.1f%% (expected: RAID 5 >= RAID 4)\n", raid5to4Ratio)
			}
		}
	}

	fmt.Println("\n================================================================================")
	fmt.Println("OBSERVATIONS & CONCLUSIONS")
	fmt.Println("================================================================================")

	fmt.Println(`
1. Performance vs Textbook Predictions:
   - File I/O overhead differs from raw hardware operations
   - fsync() calls add latency that may not perfectly match textbook models
   - Sequential block access patterns show different behavior than random access
   - Filesystem caching effects can amplify or reduce performance differences

2. Why Results May Vary from Textbook:
   - Administrative overhead: Filesystem operations have fixed costs
   - Block size effects: 4KB blocks behave differently than 512B predictions
   - Synchronous I/O: All writes use fsync for data durability
   - Test harness overhead: Loop iterations and function calls add latency

3. Key Takeaways:
   - RAID 0 should show maximum throughput
   - RAID 1 mirrors show 2x write latency for redundancy
   - RAID 4 shows parity disk bottleneck during writes
   - RAID 5 distributes parity for better write scaling
   - Storage vs redundancy trade-offs are clearly visible

For results closer to textbook predictions, consider:
   - Increasing number of blocks (larger dataset reduces relative overhead)
   - Using larger block sizes (8KB, 16KB)
   - Running multiple iterations and averaging results
   - Using unbuffered I/O if available
   - Testing on hardware with different storage characteristics
`)
}

func main() {
	flag.Parse()

	numBlocks := *numBlocksFlag
	if *fastMode {
		numBlocks = 6144
	} else if *veryFastMode {
		numBlocks = 1024
	}

	baseDir := filepath.Join(".", "raid_benchmark")

	fmt.Printf("RAID Simulation Benchmark\n")
	fmt.Printf("Block Size: %d bytes\n", BlockSize)
	fmt.Printf("Num Blocks: %d (Total Data: %.2f MB)\n", numBlocks, float64(numBlocks*BlockSize)/(1024*1024))
	fmt.Printf("Num Disks: %d\n\n", NumDisks)

	results := make([]*BenchmarkResult, 0)

	// RAID 0
	fmt.Println("\n================================================================================")
	fmt.Println("RAID 0: STRIPING")
	fmt.Println("================================================================================")
	raid0Dir := filepath.Join(baseDir, "raid0")
	os.RemoveAll(raid0Dir)
	raid0, err := NewRAID0(raid0Dir)
	if err != nil {
		fmt.Printf("Error creating RAID 0: %v\n", err)
	} else {
		result, err := runBenchmark(raid0, numBlocks)
		if err != nil {
			fmt.Printf("Error running RAID 0 benchmark: %v\n", err)
		} else {
			results = append(results, result)
		}
		raid0.Cleanup()
	}

	// RAID 1
	fmt.Println("\n================================================================================")
	fmt.Println("RAID 1: MIRRORING")
	fmt.Println("================================================================================")
	raid1Dir := filepath.Join(baseDir, "raid1")
	os.RemoveAll(raid1Dir)
	raid1, err := NewRAID1(raid1Dir)
	if err != nil {
		fmt.Printf("Error creating RAID 1: %v\n", err)
	} else {
		result, err := runBenchmark(raid1, numBlocks/2)
		if err != nil {
			fmt.Printf("Error running RAID 1 benchmark: %v\n", err)
		} else {
			results = append(results, result)
		}
		raid1.Cleanup()
	}

	// RAID 4
	fmt.Println("\n================================================================================")
	fmt.Println("RAID 4: STRIPING WITH DEDICATED PARITY")
	fmt.Println("================================================================================")
	raid4Dir := filepath.Join(baseDir, "raid4")
	os.RemoveAll(raid4Dir)
	raid4, err := NewRAID4(raid4Dir)
	if err != nil {
		fmt.Printf("Error creating RAID 4: %v\n", err)
	} else {
		result, err := runBenchmark(raid4, numBlocks*4/5)
		if err != nil {
			fmt.Printf("Error running RAID 4 benchmark: %v\n", err)
		} else {
			results = append(results, result)
		}
		raid4.Cleanup()
	}

	// RAID 5
	fmt.Println("\n================================================================================")
	fmt.Println("RAID 5: STRIPING WITH DISTRIBUTED PARITY")
	fmt.Println("================================================================================")
	raid5Dir := filepath.Join(baseDir, "raid5")
	os.RemoveAll(raid5Dir)
	raid5, err := NewRAID5(raid5Dir)
	if err != nil {
		fmt.Printf("Error creating RAID 5: %v\n", err)
	} else {
		result, err := runBenchmark(raid5, numBlocks*4/5)
		if err != nil {
			fmt.Printf("Error running RAID 5 benchmark: %v\n", err)
		} else {
			results = append(results, result)
		}
		raid5.Cleanup()
	}

	// Print results
	printResults(results)

	// Cleanup
	os.RemoveAll(baseDir)
	fmt.Println("Cleanup completed.\n")
}
