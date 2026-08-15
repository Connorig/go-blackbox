//go:build linux

package monitor

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// memoryStats Linux:读 /proc/meminfo。
func (c *Collector) memoryStats() (MemoryStats, error) {
	values := map[string]uint64{}
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryStats{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[parts[0]] = value * 1024 // kB → bytes
	}
	if err := scanner.Err(); err != nil {
		return MemoryStats{}, err
	}

	total := values["MemTotal"]
	// used = total - free - buffers - cached - sreclaimable(与 free 命令口径一致)
	free := values["MemFree"] + values["Buffers"] + values["Cached"] + values["SReclaimable"]
	used := uint64(0)
	if total > free {
		used = total - free
	}
	return MemoryStats{
		Total:        total,
		Used:         used,
		Free:         total - used,
		UsagePercent: percent(used, total),
	}, nil
}

// cpuStats Linux:读取 /proc/stat 两次采样计算使用率。
// 首次调用(无缓存)返回 0,后续调用返回自上次采样以来的平均使用率。
func (c *Collector) cpuStats() (CPUStats, error) {
	idle, total, err := readProcStat()
	if err != nil {
		return CPUStats{}, err
	}
	c.cpuLock <- struct{}{}
	defer func() { <-c.cpuLock }()

	if c.lastCPU == nil {
		c.lastCPU = &cpuCache{idle: idle, total: total, sampledAt: time.Now()}
		return CPUStats{UsagePercent: 0}, nil
	}
	prev := c.lastCPU
	now := time.Now()
	c.lastCPU = &cpuCache{idle: idle, total: total, sampledAt: now}

	totalDelta := total - prev.total
	idleDelta := idle - prev.idle
	usage := 0.0
	if totalDelta > 0 {
		usage = 100 * (1 - float64(idleDelta)/float64(totalDelta))
	}
	return CPUStats{UsagePercent: usage}, nil
}

// readProcStat 解析 /proc/stat 第一行(cpu 汇总)。
func readProcStat() (idle, total uint64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, 0, os.ErrInvalid
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, os.ErrInvalid
	}
	var values [8]uint64 // user nice system idle iowait irq softirq steal
	for i := 1; i < len(fields) && i <= 8; i++ {
		values[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
	}
	idle = values[3] + values[4] // idle + iowait
	for _, value := range values {
		total += value
	}
	return idle, total, nil
}

// diskStats Linux:根分区(statfs)。
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

// loadStats Linux:读 /proc/loadavg。
func (c *Collector) loadStats() (LoadStats, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadStats{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return LoadStats{}, os.ErrInvalid
	}
	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)
	return LoadStats{Load1: load1, Load5: load5, Load15: load15}, nil
}

// uptime Linux:读 /proc/uptime。
func (c *Collector) uptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	seconds, _ := strconv.ParseFloat(fields[0], 64)
	return int64(seconds)
}
