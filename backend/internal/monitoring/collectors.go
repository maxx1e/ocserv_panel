package monitoring

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

const (
	procStatPath    = "/proc/stat"
	procMeminfoPath = "/proc/meminfo"
)

type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

type OSReader struct{}

var _ FileReader = OSReader{}

func (r OSReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type CollectStatus string

const (
	CollectStatusAvailable   CollectStatus = "available"
	CollectStatusUnavailable CollectStatus = "unavailable"
)

type CollectResult struct {
	Samples []metrics.Sample
	Status  CollectStatus
	Message string
}

type Collector interface {
	Collect(ctx context.Context, now time.Time) CollectResult
}

type CPUCollector struct {
	mu       sync.Mutex
	reader   FileReader
	previous cpuSnapshot
	hasPrev  bool
}

var _ Collector = (*CPUCollector)(nil)

func NewCPUCollector(reader FileReader) *CPUCollector {
	return &CPUCollector{reader: reader}
}

func (c *CPUCollector) Collect(ctx context.Context, now time.Time) CollectResult {
	if err := ctx.Err(); err != nil {
		return unavailable(err.Error())
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := c.reader.ReadFile(procStatPath)
	if err != nil {
		return unavailable("read cpu stats: " + err.Error())
	}

	snapshot, err := parseCPUSnapshot(string(data))
	if err != nil {
		return unavailable(err.Error())
	}

	if !c.hasPrev {
		c.previous = snapshot
		c.hasPrev = true

		return unavailable("cpu utilization requires a previous snapshot")
	}

	if snapshot.total <= c.previous.total || snapshot.busy < c.previous.busy {
		c.previous = snapshot

		return unavailable("cpu counters did not increase")
	}

	busyDelta := snapshot.busy - c.previous.busy
	totalDelta := snapshot.total - c.previous.total
	c.previous = snapshot
	if busyDelta > totalDelta {
		return unavailable("cpu counters did not increase")
	}

	return available([]metrics.Sample{
		{
			Name:      metrics.MetricHostCPUUtilization,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(busyDelta) / float64(totalDelta) * 100,
		},
	})
}

type MemoryCollector struct {
	reader FileReader
}

var _ Collector = (*MemoryCollector)(nil)

func NewMemoryCollector(reader FileReader) *MemoryCollector {
	return &MemoryCollector{reader: reader}
}

func (c *MemoryCollector) Collect(ctx context.Context, now time.Time) CollectResult {
	if err := ctx.Err(); err != nil {
		return unavailable(err.Error())
	}

	data, err := c.reader.ReadFile(procMeminfoPath)
	if err != nil {
		return unavailable("read memory stats: " + err.Error())
	}

	total, availableMemory, err := parseMeminfo(string(data))
	if err != nil {
		return unavailable(err.Error())
	}
	if total <= 0 || availableMemory > total {
		return unavailable("invalid memory counters")
	}

	used := total - availableMemory

	return available([]metrics.Sample{
		{
			Name:      metrics.MetricHostMemoryUtilization,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(used) / float64(total) * 100,
		},
	})
}

type InterfaceCollector struct {
	mu       sync.Mutex
	reader   FileReader
	iface    string
	rxMetric metrics.MetricName
	txMetric metrics.MetricName
	prevRX   uint64
	prevTX   uint64
	prevTime time.Time
	hasPrev  bool
}

var _ Collector = (*InterfaceCollector)(nil)

func NewInterfaceCollector(
	reader FileReader,
	iface string,
	rxMetric metrics.MetricName,
	txMetric metrics.MetricName,
) *InterfaceCollector {
	return &InterfaceCollector{
		reader:   reader,
		iface:    iface,
		rxMetric: rxMetric,
		txMetric: txMetric,
	}
}

func (c *InterfaceCollector) Collect(ctx context.Context, now time.Time) CollectResult {
	if err := ctx.Err(); err != nil {
		return unavailable(err.Error())
	}
	if c.iface == "" {
		return unavailable("interface name is required")
	}

	rxBytes, err := c.readCounter(ctx, "rx_bytes")
	if err != nil {
		return unavailable(err.Error())
	}

	txBytes, err := c.readCounter(ctx, "tx_bytes")
	if err != nil {
		return unavailable(err.Error())
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasPrev {
		c.prevRX = rxBytes
		c.prevTX = txBytes
		c.prevTime = now
		c.hasPrev = true
		return unavailable("rate calculation requires a previous sample")
	}

	// Detect counter wrap: new value less than previous means the kernel
	// counter wrapped around. Reset baseline and skip this sample.
	if rxBytes < c.prevRX || txBytes < c.prevTX {
		c.prevRX = rxBytes
		c.prevTX = txBytes
		c.prevTime = now
		return unavailable("counter wrap detected, resetting baseline")
	}

	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed <= 0 {
		return unavailable("elapsed time must be positive")
	}

	rxRate := float64(rxBytes-c.prevRX) / elapsed
	txRate := float64(txBytes-c.prevTX) / elapsed

	c.prevRX = rxBytes
	c.prevTX = txBytes
	c.prevTime = now

	labels := map[string]string{"interface": c.iface}

	return available([]metrics.Sample{
		{
			Name:      c.rxMetric,
			Labels:    labels,
			Timestamp: now,
			Value:     rxRate,
		},
		{
			Name:      c.txMetric,
			Labels:    labels,
			Timestamp: now,
			Value:     txRate,
		},
	})
}

func (c *InterfaceCollector) readCounter(ctx context.Context, name string) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/sys/class/net/%s/statistics/%s", c.iface, name)
	data, err := c.reader.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read interface counter: %w", err)
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse interface counter: %w", err)
	}

	return value, nil
}

type cpuSnapshot struct {
	busy  uint64
	total uint64
}

func parseCPUSnapshot(data string) (cpuSnapshot, error) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "cpu" {
			continue
		}
		if len(fields) < 5 {
			return cpuSnapshot{}, fmt.Errorf("parse cpu stats: expected at least 4 counters")
		}

		counters := make([]uint64, 0, len(fields)-1)
		for _, field := range fields[1:] {
			counter, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return cpuSnapshot{}, fmt.Errorf("parse cpu stats: %w", err)
			}
			counters = append(counters, counter)
		}

		var total uint64
		for _, counter := range counters {
			total += counter
		}

		idle := counters[3]
		if len(counters) > 4 {
			idle += counters[4]
		}

		if total < idle {
			return cpuSnapshot{}, fmt.Errorf("parse cpu stats: idle exceeds total")
		}

		return cpuSnapshot{busy: total - idle, total: total}, nil
	}

	return cpuSnapshot{}, fmt.Errorf("parse cpu stats: missing aggregate cpu line")
}

func parseMeminfo(data string) (uint64, uint64, error) {
	var total uint64
	var availableMemory uint64
	var hasTotal bool
	var hasAvailableMemory bool

	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parse memory stats: %w", err)
		}

		switch fields[0] {
		case "MemTotal:":
			total = value
			hasTotal = true
		case "MemAvailable:":
			availableMemory = value
			hasAvailableMemory = true
		}
	}

	if !hasTotal || total == 0 {
		return 0, 0, fmt.Errorf("parse memory stats: missing MemTotal")
	}
	if !hasAvailableMemory {
		return 0, 0, fmt.Errorf("parse memory stats: missing MemAvailable")
	}

	return total, availableMemory, nil
}

func available(samples []metrics.Sample) CollectResult {
	return CollectResult{
		Samples: samples,
		Status:  CollectStatusAvailable,
	}
}

func unavailable(message string) CollectResult {
	return CollectResult{
		Samples: []metrics.Sample{},
		Status:  CollectStatusUnavailable,
		Message: message,
	}
}
