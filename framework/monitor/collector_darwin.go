//go:build darwin

package monitor

import (
	"encoding/binary"
	"syscall"
	"time"
)

// sysctlUint64 读取整型 sysctl(Go darwin syscall.Sysctl 返回 string 字节流)。
func sysctlUint64(name string) (uint64, error) {
	value, err := syscall.Sysctl(name)
	if err != nil {
		return 0, err
	}
	if len(value) < 8 {
		return 0, syscall.EINVAL
	}
	return binary.LittleEndian.Uint64([]byte(value)), nil
}

// sysctlBytes 读取原始字节流 sysctl。
func sysctlBytes(name string) ([]byte, error) {
	value, err := syscall.Sysctl(name)
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// memoryStats Darwin:sysctl hw.memsize + vm.page_free_count/vm.page_size。
// free = 空闲页 × 页大小(与 top 的 Free 口径近似,不含 inactive 回收页)。
func (c *Collector) memoryStats() (MemoryStats, error) {
	total, err := sysctlUint64("hw.memsize")
	if err != nil {
		return MemoryStats{}, err
	}
	freePages, err := sysctlUint64("vm.page_free_count")
	if err != nil {
		return MemoryStats{}, err
	}
	pageSize, err := sysctlUint64("vm.page_size")
	if err != nil {
		return MemoryStats{}, err
	}
	free := freePages * pageSize
	used := uint64(0)
	if total > free {
		used = total - free
	}
	return MemoryStats{
		Total:        total,
		Used:         used,
		Free:         free,
		UsagePercent: percent(used, total),
	}, nil
}

// cpuStats Darwin:sysctl kern.cp_time 两次采样(采样逻辑复用 sampleCPU)。
// 首次调用(无缓存)返回 0,后续调用返回自上次采样以来的平均使用率。
func (c *Collector) cpuStats() (CPUStats, error) {
	idle, total, err := readKernCPTime()
	if err != nil {
		return CPUStats{}, err
	}
	return c.sampleCPU(idle, total)
}

// readKernCPTime 读 kern.cp_time:user nice system idle(4 × uint32,小端)。
func readKernCPTime() (idle, total uint64, err error) {
	data, err := sysctlBytes("kern.cp_time")
	if err != nil {
		return 0, 0, err
	}
	if len(data) < 16 {
		return 0, 0, syscall.EINVAL
	}
	user := binary.LittleEndian.Uint32(data[0:4])
	nice := binary.LittleEndian.Uint32(data[4:8])
	system := binary.LittleEndian.Uint32(data[8:12])
	idle32 := binary.LittleEndian.Uint32(data[12:16])
	idle = uint64(idle32)
	total = uint64(user) + uint64(nice) + uint64(system) + uint64(idle32)
	return idle, total, nil
}

// diskStats Darwin:根分区(statfs)。
func (c *Collector) diskStats() (DiskStats, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return DiskStats{}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - available
	return DiskStats{
		Total:        total,
		Used:         used,
		Free:         available,
		UsagePercent: percent(used, total),
	}, nil
}

// loadStats Darwin:sysctl vm.loadavg(3 × fixpt_t uint32,定点缩放 65536)。
func (c *Collector) loadStats() (LoadStats, error) {
	data, err := sysctlBytes("vm.loadavg")
	if err != nil {
		return LoadStats{}, err
	}
	if len(data) < 12 {
		return LoadStats{}, syscall.EINVAL
	}
	const scale = 65536.0
	return LoadStats{
		Load1:  float64(binary.LittleEndian.Uint32(data[0:4])) / scale,
		Load5:  float64(binary.LittleEndian.Uint32(data[4:8])) / scale,
		Load15: float64(binary.LittleEndian.Uint32(data[8:12])) / scale,
	}, nil
}

// uptime Darwin:sysctl kern.boottime(timeval) 距当前时间。
func (c *Collector) uptime() int64 {
	data, err := sysctlBytes("kern.boottime")
	if err != nil || len(data) < 8 {
		return 0
	}
	bootSeconds := int64(binary.LittleEndian.Uint64(data[0:8]))
	return time.Now().Unix() - bootSeconds
}
