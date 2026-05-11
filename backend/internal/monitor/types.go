package monitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type MetricName string

const (
	MetricHostCPUUtilization      MetricName = "host_cpu_utilization"
	MetricHostMemoryUtilization   MetricName = "host_memory_utilization"
	MetricNetInterfaceRXBytes     MetricName = "net_interface_rx_bytes"
	MetricNetInterfaceTXBytes     MetricName = "net_interface_tx_bytes"
	MetricVPNInterfaceRXBytes     MetricName = "vpn_interface_rx_bytes"
	MetricVPNInterfaceTXBytes     MetricName = "vpn_interface_tx_bytes"
	MetricUserRXBytes             MetricName = "user_rx_bytes"
	MetricUserTXBytes             MetricName = "user_tx_bytes"
	MetricActiveSessions          MetricName = "active_sessions"
	MetricConnectionRate          MetricName = "connection_rate"
	MetricConcurrentSessions      MetricName = "concurrent_sessions"
	MetricConcurrentSessionsLimit MetricName = "concurrent_sessions_limit"
)

type State string

const (
	StateCollecting    State = "collecting"
	StateUnavailable   State = "unavailable"
	maxRangeDataPoints       = 1500
)

var (
	ErrInvalidMetric = errors.New("invalid metric")
	ErrInvalidRange  = errors.New("invalid range")
	ErrInvalidSample = errors.New("invalid sample")
	ErrInvalidWindow = errors.New("invalid window")
	ErrInvalidStep   = errors.New("invalid step")
	ErrStoreClosed   = errors.New("store closed")
)

var allowedWindows = map[string]time.Duration{
	"15m": time.Minute * 15,
	"1h":  time.Hour,
	"6h":  time.Hour * 6,
	"24h": time.Hour * 24,
	"7d":  time.Hour * 24 * 7,
	"30d": time.Hour * 24 * 30,
}

var allowedSteps = map[string]time.Duration{
	"15s": time.Second * 15,
	"60s": time.Minute,
	"5m":  time.Minute * 5,
	"1h":  time.Hour,
}

type MetricDefinition struct {
	Name        MetricName `json:"name"`
	Description string     `json:"description"`
	State       State      `json:"state"`
	Reason      string     `json:"reason,omitempty"`
}

type Catalog struct {
	mu      sync.RWMutex
	metrics map[MetricName]MetricDefinition
}

func DefaultCatalog() *Catalog {
	defs := []MetricDefinition{
		collectingMetric(MetricHostCPUUtilization, "host CPU utilization"),
		collectingMetric(MetricHostMemoryUtilization, "host memory utilization"),
		collectingMetric(MetricNetInterfaceRXBytes, "network interface received bytes"),
		collectingMetric(MetricNetInterfaceTXBytes, "network interface transmitted bytes"),
		unavailableWithReason(MetricVPNInterfaceRXBytes, "VPN interface received bytes", "waiting for VPN interface"),
		unavailableWithReason(MetricVPNInterfaceTXBytes, "VPN interface transmitted bytes", "waiting for VPN interface"),
		unavailableMetric(MetricUserRXBytes, "user received bytes"),
		unavailableMetric(MetricUserTXBytes, "user transmitted bytes"),
		collectingMetric(MetricActiveSessions, "active sessions"),
		collectingMetric(MetricConnectionRate, "connection rate"),
		collectingMetric(MetricConcurrentSessions, "concurrent sessions"),
		collectingMetric(MetricConcurrentSessionsLimit, "concurrent sessions limit"),
	}

	catalog := &Catalog{metrics: make(map[MetricName]MetricDefinition, len(defs))}
	for _, metric := range defs {
		catalog.metrics[metric.Name] = metric
	}

	return catalog
}

// Find returns the metric definition for the given name.
func (c *Catalog) Find(name MetricName) (MetricDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	metric, ok := c.metrics[name]
	return metric, ok
}

// SetState updates the state and reason of a metric in the catalog.
// If the metric does not exist in the catalog, the call is a no-op.
func (c *Catalog) SetState(name MetricName, state State, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if def, ok := c.metrics[name]; ok {
		def.State = state
		def.Reason = reason
		c.metrics[name] = def
	}
}

// List returns a snapshot of all metric definitions.
func (c *Catalog) List() []MetricDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	defs := make([]MetricDefinition, 0, len(c.metrics))
	for _, d := range c.metrics {
		defs = append(defs, d)
	}
	return defs
}

type Sample struct {
	Name      MetricName
	Labels    map[string]string
	Timestamp time.Time
	Value     float64
}

type Point struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type Series struct {
	Metric MetricName        `json:"metric"`
	Labels map[string]string `json:"labels"`
	Points []Point           `json:"points"`
}

type Query struct {
	Metric MetricName
	Labels map[string]string
	Start  time.Time
	End    time.Time
	Step   time.Duration
}

type Store interface {
	Append(ctx context.Context, samples []Sample) error
	QueryRange(ctx context.Context, query Query) (Series, error)
	QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error)
	// QueryAllLabels returns all distinct label sets ever seen for the given metric.
	QueryAllLabels(ctx context.Context, metric MetricName) ([]map[string]string, error)
	Close() error
}

func ValidateWindowAndStep(window string, step string) (time.Duration, time.Duration, error) {
	windowDuration, ok := allowedWindows[window]
	if !ok {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidWindow, window)
	}

	stepDuration, ok := allowedSteps[step]
	if !ok {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidStep, step)
	}

	if windowDuration/stepDuration > maxRangeDataPoints {
		return 0, 0, fmt.Errorf("%w: too many points", ErrInvalidStep)
	}

	return windowDuration, stepDuration, nil
}

func ValidateMetric(name MetricName) error {
	if _, ok := DefaultCatalog().Find(name); !ok {
		return fmt.Errorf("%w: %q", ErrInvalidMetric, name)
	}

	return nil
}

func collectingMetric(name MetricName, description string) MetricDefinition {
	return MetricDefinition{
		Name:        name,
		Description: description,
		State:       StateCollecting,
	}
}

func unavailableMetric(name MetricName, description string) MetricDefinition {
	return MetricDefinition{
		Name:        name,
		Description: description,
		State:       StateUnavailable,
		Reason:      "ocserv collection is not implemented yet",
	}
}

func unavailableWithReason(name MetricName, description, reason string) MetricDefinition {
	return MetricDefinition{
		Name:        name,
		Description: description,
		State:       StateUnavailable,
		Reason:      reason,
	}
}

func cloneLabels(labels map[string]string) map[string]string {
	cloned := make(map[string]string, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}

	return cloned
}

// labelsKey returns a stable string key for a label set, used for deduplication.
func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(labels))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}
