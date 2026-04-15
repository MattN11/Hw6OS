package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	NumDisks       = 5
	BlockSize      = 4096 // 4KB blocks
	DefaultDiskDir = "disks"
)

// RAID defines the interface for all RAID levels
type RAID interface {
	Write(blockNum int, data []byte) error
	Read(blockNum int) ([]byte, error)
	GetCapacity() int64 // Returns total capacity in bytes
	Cleanup() error
	GetName() string
}

// DiskManager handles reading/writing to physical disk files
type DiskManager struct {
	disks   []*os.File
	diskDir string
}

// NewDiskManager creates a new disk manager with specified number of disks
func NewDiskManager(numDisks int, diskDir string) (*DiskManager, error) {
	dm := &DiskManager{
		disks:   make([]*os.File, numDisks),
		diskDir: diskDir,
	}

	// Create disks directory if it doesn't exist
	if err := os.MkdirAll(diskDir, 0755); err != nil {
		return nil, err
	}

	// Open or create disk files
	for i := 0; i < numDisks; i++ {
		diskPath := filepath.Join(diskDir, fmt.Sprintf("disk%d.dat", i))
		f, err := os.OpenFile(diskPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			// Cleanup on error
			dm.Close()
			return nil, err
		}
		dm.disks[i] = f
	}

	return dm, nil
}

// WriteToDisk writes data to a specific disk at the given block offset
func (dm *DiskManager) WriteToDisk(diskNum int, blockNum int, data []byte) error {
	if diskNum < 0 || diskNum >= len(dm.disks) {
		return fmt.Errorf("invalid disk number: %d", diskNum)
	}
	if dm.disks[diskNum] == nil {
		return fmt.Errorf("disk %d is not open", diskNum)
	}

	offset := int64(blockNum) * int64(BlockSize)
	if _, err := dm.disks[diskNum].WriteAt(data, offset); err != nil {
		return err
	}
	// Flush to disk to simulate real disk write
	return dm.disks[diskNum].Sync()
}

// ReadFromDisk reads data from a specific disk at the given block offset
func (dm *DiskManager) ReadFromDisk(diskNum int, blockNum int) ([]byte, error) {
	if diskNum < 0 || diskNum >= len(dm.disks) {
		return nil, fmt.Errorf("invalid disk number: %d", diskNum)
	}
	if dm.disks[diskNum] == nil {
		return nil, fmt.Errorf("disk %d is not open", diskNum)
	}

	buffer := make([]byte, BlockSize)
	offset := int64(blockNum) * int64(BlockSize)
	n, err := dm.disks[diskNum].ReadAt(buffer, offset)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buffer[:n], nil
}

// Close closes all disk files
func (dm *DiskManager) Close() error {
	var lastErr error
	for i := range dm.disks {
		if dm.disks[i] != nil {
			if err := dm.disks[i].Close(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// CleanupDiskFiles removes all disk files
func (dm *DiskManager) CleanupDiskFiles() error {
	var lastErr error
	for i := 0; i < len(dm.disks); i++ {
		diskPath := filepath.Join(dm.diskDir, fmt.Sprintf("disk%d.dat", i))
		if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	return lastErr
}

// ===== RAID 0: Striping (No Redundancy) =====

type RAID0 struct {
	dm *DiskManager
}

func NewRAID0(diskDir string) (*RAID0, error) {
	dm, err := NewDiskManager(NumDisks, diskDir)
	if err != nil {
		return nil, err
	}
	return &RAID0{dm: dm}, nil
}

func (r *RAID0) Write(blockNum int, data []byte) error {
	if len(data) != BlockSize {
		return fmt.Errorf("data size must be %d bytes", BlockSize)
	}
	// In RAID 0, stripe the block across all disks
	// blockNum is the logical block number
	// Physical placement: disk = blockNum % NumDisks, physical_block = blockNum / NumDisks
	physicalDisk := blockNum % NumDisks
	physicalBlock := blockNum / NumDisks
	return r.dm.WriteToDisk(physicalDisk, physicalBlock, data)
}

func (r *RAID0) Read(blockNum int) ([]byte, error) {
	physicalDisk := blockNum % NumDisks
	physicalBlock := blockNum / NumDisks
	return r.dm.ReadFromDisk(physicalDisk, physicalBlock)
}

func (r *RAID0) GetCapacity() int64 {
	return int64(NumDisks) * int64(BlockSize) * int64(1000000) // Large enough for benchmarks
}

func (r *RAID0) Cleanup() error {
	r.dm.Close()
	return r.dm.CleanupDiskFiles()
}

func (r *RAID0) GetName() string {
	return "RAID 0 (Striping)"
}

// ===== RAID 1: Mirroring =====

type RAID1 struct {
	dm *DiskManager
}

func NewRAID1(diskDir string) (*RAID1, error) {
	dm, err := NewDiskManager(NumDisks, diskDir)
	if err != nil {
		return nil, err
	}
	return &RAID1{dm: dm}, nil
}

func (r *RAID1) Write(blockNum int, data []byte) error {
	if len(data) != BlockSize {
		return fmt.Errorf("data size must be %d bytes", BlockSize)
	}
	// In RAID 1, mirror to 2 disks
	// We'll use disks 0-1, 2-3 as mirror pairs, and disk 4 as extra
	// blockNum maps to which mirror pair
	mirror := blockNum % (NumDisks / 2) // 2 pairs (0-1, 2-3)
	blockInMirror := blockNum / (NumDisks / 2)

	// Write to both disks in the mirror pair
	disk1 := mirror * 2
	disk2 := mirror*2 + 1

	if err := r.dm.WriteToDisk(disk1, blockInMirror, data); err != nil {
		return err
	}
	return r.dm.WriteToDisk(disk2, blockInMirror, data)
}

func (r *RAID1) Read(blockNum int) ([]byte, error) {
	mirror := blockNum % (NumDisks / 2)
	blockInMirror := blockNum / (NumDisks / 2)

	// Read from first disk in mirror pair
	disk1 := mirror * 2
	return r.dm.ReadFromDisk(disk1, blockInMirror)
}

func (r *RAID1) GetCapacity() int64 {
	// RAID 1 has 50% effective capacity (mirrored)
	return int64(NumDisks/2) * int64(BlockSize) * int64(1000000)
}

func (r *RAID1) Cleanup() error {
	r.dm.Close()
	return r.dm.CleanupDiskFiles()
}

func (r *RAID1) GetName() string {
	return "RAID 1 (Mirroring)"
}

// ===== RAID 4: Block-level Striping with Dedicated Parity =====

type RAID4 struct {
	dm       *DiskManager
	dataDisk int // Number of data disks (NumDisks - 1)
}

func NewRAID4(diskDir string) (*RAID4, error) {
	dm, err := NewDiskManager(NumDisks, diskDir)
	if err != nil {
		return nil, err
	}
	return &RAID4{
		dm:       dm,
		dataDisk: NumDisks - 1, // Last disk is for parity
	}, nil
}

func (r *RAID4) Write(blockNum int, data []byte) error {
	if len(data) != BlockSize {
		return fmt.Errorf("data size must be %d bytes", BlockSize)
	}

	// Determine stripe number and position
	stripeNum := blockNum / r.dataDisk
	posInStripe := blockNum % r.dataDisk

	// Write data to the appropriate disk
	if err := r.dm.WriteToDisk(posInStripe, stripeNum, data); err != nil {
		return err
	}

	// Calculate and write parity to the dedicated parity disk (last disk)
	parity := r.calculateStripeXOR(stripeNum)
	return r.dm.WriteToDisk(r.dataDisk, stripeNum, parity)
}

func (r *RAID4) Read(blockNum int) ([]byte, error) {
	stripeNum := blockNum / r.dataDisk
	posInStripe := blockNum % r.dataDisk
	return r.dm.ReadFromDisk(posInStripe, stripeNum)
}

func (r *RAID4) calculateStripeXOR(stripeNum int) []byte {
	parity := make([]byte, BlockSize)

	for diskNum := 0; diskNum < r.dataDisk; diskNum++ {
		data, err := r.dm.ReadFromDisk(diskNum, stripeNum)
		if err != nil || data == nil {
			continue
		}
		for i := 0; i < len(data) && i < BlockSize; i++ {
			parity[i] ^= data[i]
		}
	}
	return parity
}

func (r *RAID4) GetCapacity() int64 {
	return int64(r.dataDisk) * int64(BlockSize) * int64(1000000)
}

func (r *RAID4) Cleanup() error {
	r.dm.Close()
	return r.dm.CleanupDiskFiles()
}

func (r *RAID4) GetName() string {
	return "RAID 4 (Striping + Dedicated Parity)"
}

// ===== RAID 5: Block-level Striping with Distributed Parity =====

type RAID5 struct {
	dm       *DiskManager
	dataDisk int // Number of data disks (NumDisks - 1)
}

func NewRAID5(diskDir string) (*RAID5, error) {
	dm, err := NewDiskManager(NumDisks, diskDir)
	if err != nil {
		return nil, err
	}
	return &RAID5{
		dm:       dm,
		dataDisk: NumDisks - 1, // One disk worth of parity space distributed
	}, nil
}

func (r *RAID5) Write(blockNum int, data []byte) error {
	if len(data) != BlockSize {
		return fmt.Errorf("data size must be %d bytes", BlockSize)
	}

	stripeNum := blockNum / r.dataDisk
	posInStripe := blockNum % r.dataDisk

	// In RAID 5, parity disk rotates: parity_disk = (stripeNum) % NumDisks
	// Data disks are arranged in a circular pattern
	diskNum := (posInStripe + stripeNum) % NumDisks

	// Write data
	if err := r.dm.WriteToDisk(diskNum, stripeNum, data); err != nil {
		return err
	}

	// Calculate and write parity
	parity := r.calculateStripeXOR(stripeNum)
	parityDisk := (stripeNum) % NumDisks
	if parityDisk == diskNum {
		// Avoid writing to same disk twice; use next disk
		parityDisk = (parityDisk + 1) % NumDisks
	}
	return r.dm.WriteToDisk(parityDisk, stripeNum, parity)
}

func (r *RAID5) Read(blockNum int) ([]byte, error) {
	stripeNum := blockNum / r.dataDisk
	posInStripe := blockNum % r.dataDisk
	diskNum := (posInStripe + stripeNum) % NumDisks
	return r.dm.ReadFromDisk(diskNum, stripeNum)
}

func (r *RAID5) calculateStripeXOR(stripeNum int) []byte {
	parity := make([]byte, BlockSize)

	// Only XOR data blocks, not parity blocks
	// Each stripe has dataDisk data blocks distributed across NumDisks
	for pos := 0; pos < r.dataDisk; pos++ {
		diskNum := (pos + stripeNum) % NumDisks
		data, err := r.dm.ReadFromDisk(diskNum, stripeNum)
		if err != nil || data == nil {
			continue
		}
		for j := 0; j < len(data) && j < BlockSize; j++ {
			parity[j] ^= data[j]
		}
	}
	return parity
}

func (r *RAID5) GetCapacity() int64 {
	return int64(r.dataDisk) * int64(BlockSize) * int64(1000000)
}

func (r *RAID5) Cleanup() error {
	r.dm.Close()
	return r.dm.CleanupDiskFiles()
}

func (r *RAID5) GetName() string {
	return "RAID 5 (Striping + Distributed Parity)"
}
