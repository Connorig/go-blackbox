//go:build windows

package monitor

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx       = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes             = kernel32.NewProc("GetSystemTimes")
	procGetDiskFreeSpaceExW        = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetTickCount64             = kernel32.NewProc("GetTickCount64")
)

// memoryStats Windows:GlobalMemoryStatusEx。
func (c *Collector) memoryStats() (MemoryStats, error) {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	result, _, callErr := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return MemoryStats{}, callErr
	}
	used := status.TotalPhys - status.AvailPhys
	return MemoryStats{
		Total:        status.TotalPhys,
		Used:         used,
		Free:         status.AvailPhys,
		UsagePercent: percent(used, status.TotalPhys),
	}, nil
}

// memoryStatusEx 对齐 MEMORYSTATUSEX 结构(前 4 个字段足够)。
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// cpuStats Windows:GetSystemTimes 采样差值。
// 首次调用返回 0;采样间隔过短(<100ms)时复用上次值。
func (c *Collector) cpuStats() (CPUStats, error) {
	idle, kernel, user, err := readSystemTimes()
	if err != nil {
		return CPUStats{}, err
	}
	// total 包含 idle,使用率 = (total - idleDelta) / totalDelta
	idleTotal := idle
	allTotal := kernel + user

	c.cpuLock <- struct{}{}
	defer func() { <-c.cpuLock }()

	if c.lastCPU == nil {
		c.lastCPU = &cpuCache{idle: idleTotal, total: allTotal, sampledAt: time.Now()}
		return CPUStats{UsagePercent: 0}, nil
	}
	prev := c.lastCPU
	if time.Since(prev.sampledAt) < 100*time.Millisecond {
		return CPUStats{UsagePercent: 0}, nil
	}
	c.lastCPU = &cpuCache{idle: idleTotal, total: allTotal, sampledAt: time.Now()}

	totalDelta := allTotal - prev.total
	idleDelta := idleTotal - prev.idle
	usage := 0.0
	if totalDelta > 0 {
		usage = 100 * (1 - float64(idleDelta)/float64(totalDelta))
	}
	return CPUStats{UsagePercent: usage}, nil
}

// readSystemTimes 读取内核/用户/空闲时间(100ns 单位)。
func readSystemTimes() (idle, kernel, user uint64, err error) {
	var idleTime, kernelTime, userTime windows.Filetime
	result, _, callErr := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)))
	if result == 0 {
		return 0, 0, 0, callErr
	}
	return filetimeToUint64(idleTime), filetimeToUint64(kernelTime), filetimeToUint64(userTime), nil
}

func filetimeToUint64(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

// diskStats Windows:根分区 GetDiskFreeSpaceExW。
func (c *Collector) diskStats() (DiskStats, error) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	root, err := windows.UTF16PtrFromString(`C:\`)
	if err != nil {
		return DiskStats{}, err
	}
	result, _, callErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)))
	if result == 0 {
		return DiskStats{}, callErr
	}
	used := totalBytes - freeBytesAvailable
	return DiskStats{
		Total:        totalBytes,
		Used:         used,
		Free:         freeBytesAvailable,
		UsagePercent: percent(used, totalBytes),
	}, nil
}

// loadStats Windows:无系统负载概念,返回 0。
func (c *Collector) loadStats() (LoadStats, error) {
	return LoadStats{}, nil
}

// uptime Windows:GetTickCount64。
func (c *Collector) uptime() int64 {
	result, _, _ := procGetTickCount64.Call()
	return int64(result) / 1000
}
