package monitoring

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

func TestCPUCollector(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	reader := &fakeFileReader{
		files: map[string][]string{
			"/proc/stat": {
				"cpu  100 0 100 800 0 0 0 0 0 0\n",
				"cpu  150 0 150 850 0 0 0 0 0 0\n",
			},
		},
	}
	collector := NewCPUCollector(reader)

	first := collector.Collect(context.Background(), now)
	if first.Status != CollectStatusUnavailable {
		t.Fatalf("first status = %q, want %q", first.Status, CollectStatusUnavailable)
	}
	if len(first.Samples) != 0 {
		t.Fatalf("first samples length = %d, want 0", len(first.Samples))
	}
	if first.Message == "" {
		t.Fatal("first message is empty")
	}

	second := collector.Collect(context.Background(), now.Add(time.Second))
	if second.Status != CollectStatusAvailable {
		t.Fatalf("second status = %q, want %q", second.Status, CollectStatusAvailable)
	}
	if len(second.Samples) != 1 {
		t.Fatalf("second samples length = %d, want 1", len(second.Samples))
	}

	sample := second.Samples[0]
	if sample.Name != metrics.MetricHostCPUUtilization {
		t.Fatalf("sample name = %q, want %q", sample.Name, metrics.MetricHostCPUUtilization)
	}
	if sample.Timestamp != now.Add(time.Second) {
		t.Fatalf("sample timestamp = %s, want %s", sample.Timestamp, now.Add(time.Second))
	}
	// CPU utilization is emitted as a 0-100 percentage for UI readability.
	if sample.Value != 66.66666666666666 {
		t.Fatalf("sample value = %v, want %v", sample.Value, 66.66666666666666)
	}
}

func TestCPUCollector_MalformedInputUnavailable(t *testing.T) {
	t.Parallel()

	collector := NewCPUCollector(&fakeFileReader{
		files: map[string][]string{
			"/proc/stat": {"not cpu data\n"},
		},
	})

	result := collector.Collect(context.Background(), time.Now())
	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
	if len(result.Samples) != 0 {
		t.Fatalf("samples length = %d, want 0", len(result.Samples))
	}
	if result.Message == "" {
		t.Fatal("message is empty")
	}
}

func TestCPUCollector_DecreasingCountersUnavailableAndRecoverable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewCPUCollector(&fakeFileReader{
		files: map[string][]string{
			"/proc/stat": {
				"cpu  100 0 100 800 0 0 0 0 0 0\n",
				"cpu  90 0 90 800 0 0 0 0 0 0\n",
				"cpu  120 0 120 860 0 0 0 0 0 0\n",
			},
		},
	})

	first := collector.Collect(context.Background(), now)
	if first.Status != CollectStatusUnavailable {
		t.Fatalf("first status = %q, want %q", first.Status, CollectStatusUnavailable)
	}

	second := collector.Collect(context.Background(), now.Add(time.Second))
	if second.Status != CollectStatusUnavailable {
		t.Fatalf("second status = %q, want %q", second.Status, CollectStatusUnavailable)
	}
	if len(second.Samples) != 0 {
		t.Fatalf("second samples length = %d, want 0", len(second.Samples))
	}

	third := collector.Collect(context.Background(), now.Add(2*time.Second))
	if third.Status != CollectStatusAvailable {
		t.Fatalf("third status = %q, want %q; message = %q", third.Status, CollectStatusAvailable, third.Message)
	}
	if len(third.Samples) != 1 {
		t.Fatalf("third samples length = %d, want 1", len(third.Samples))
	}
}

func TestCPUCollector_ConcurrentCollectIsRaceFree(t *testing.T) {
	t.Parallel()

	collector := NewCPUCollector(&sequenceFileReader{
		path: procStatPath,
		makeData: func(call int) string {
			return fmt.Sprintf("cpu  %d 0 %d %d 0 0 0 0 0 0\n", 100+call, 100+call, 800+call)
		},
	})

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()

			result := collector.Collect(context.Background(), time.Unix(int64(offset), 0).UTC())
			if result.Status == CollectStatusAvailable && len(result.Samples) != 1 {
				t.Errorf("available samples length = %d, want 1", len(result.Samples))
			}
		}(i)
	}
	wg.Wait()
}

func TestMemoryCollector(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewMemoryCollector(&fakeFileReader{
		files: map[string][]string{
			"/proc/meminfo": {"MemTotal:       1000 kB\nMemAvailable:    250 kB\n"},
		},
	})

	result := collector.Collect(context.Background(), now)
	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusAvailable)
	}
	if len(result.Samples) != 1 {
		t.Fatalf("samples length = %d, want 1", len(result.Samples))
	}

	sample := result.Samples[0]
	if sample.Name != metrics.MetricHostMemoryUtilization {
		t.Fatalf("sample name = %q, want %q", sample.Name, metrics.MetricHostMemoryUtilization)
	}
	if sample.Timestamp != now {
		t.Fatalf("sample timestamp = %s, want %s", sample.Timestamp, now)
	}
	if sample.Value != 75 {
		t.Fatalf("sample value = %v, want 75", sample.Value)
	}
}

func TestMemoryCollector_ZeroMemAvailableIsFullUtilization(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewMemoryCollector(&fakeFileReader{
		files: map[string][]string{
			"/proc/meminfo": {"MemTotal:       1000 kB\nMemAvailable:      0 kB\n"},
		},
	})

	result := collector.Collect(context.Background(), now)
	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want %q; message = %q", result.Status, CollectStatusAvailable, result.Message)
	}
	if len(result.Samples) != 1 {
		t.Fatalf("samples length = %d, want 1", len(result.Samples))
	}
	if result.Samples[0].Value != 100 {
		t.Fatalf("sample value = %v, want 100", result.Samples[0].Value)
	}
}

func TestMemoryCollector_MissingRequiredFieldsUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing MemTotal",
			data: "MemAvailable:    250 kB\n",
		},
		{
			name: "missing MemAvailable",
			data: "MemTotal:       1000 kB\n",
		},
		{
			name: "malformed MemTotal",
			data: "MemTotal:       nope kB\nMemAvailable:    250 kB\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			collector := NewMemoryCollector(&fakeFileReader{
				files: map[string][]string{
					"/proc/meminfo": {tt.data},
				},
			})

			result := collector.Collect(context.Background(), time.Now())
			if result.Status != CollectStatusUnavailable {
				t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
			}
			if len(result.Samples) != 0 {
				t.Fatalf("samples length = %d, want 0", len(result.Samples))
			}
		})
	}
}

func TestInterfaceCollector(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewInterfaceCollector(
		&fakeFileReader{
			files: map[string][]string{
				"/sys/class/net/eth0/statistics/rx_bytes": {"123\n"},
				"/sys/class/net/eth0/statistics/tx_bytes": {"456\n"},
			},
		},
		"eth0",
		metrics.MetricNetInterfaceRXBytes,
		metrics.MetricNetInterfaceTXBytes,
	)

	result := collector.Collect(context.Background(), now)
	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusAvailable)
	}
	if len(result.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(result.Samples))
	}

	assertInterfaceSample(t, result.Samples[0], metrics.MetricNetInterfaceRXBytes, now, 123, "eth0")
	assertInterfaceSample(t, result.Samples[1], metrics.MetricNetInterfaceTXBytes, now, 456, "eth0")
}

func TestInterfaceCollector_UnavailableInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		iface  string
		reader *fakeFileReader
	}{
		{
			name:   "empty interface name",
			iface:  "",
			reader: &fakeFileReader{files: map[string][]string{}},
		},
		{
			name:   "unreadable rx file",
			iface:  "eth0",
			reader: &fakeFileReader{files: map[string][]string{}},
		},
		{
			name:  "malformed tx file",
			iface: "eth0",
			reader: &fakeFileReader{
				files: map[string][]string{
					"/sys/class/net/eth0/statistics/rx_bytes": {"123\n"},
					"/sys/class/net/eth0/statistics/tx_bytes": {"nope\n"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			collector := NewInterfaceCollector(
				tt.reader,
				tt.iface,
				metrics.MetricNetInterfaceRXBytes,
				metrics.MetricNetInterfaceTXBytes,
			)

			result := collector.Collect(context.Background(), time.Now())
			if result.Status != CollectStatusUnavailable {
				t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
			}
			if len(result.Samples) != 0 {
				t.Fatalf("samples length = %d, want 0", len(result.Samples))
			}
		})
	}
}

func TestInterfaceCollector_ContextCanceledAfterRXSkipsTX(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelAfterReadFileReader{
		cancel: cancel,
		files: map[string]string{
			"/sys/class/net/eth0/statistics/rx_bytes": "123\n",
			"/sys/class/net/eth0/statistics/tx_bytes": "456\n",
		},
	}
	collector := NewInterfaceCollector(
		reader,
		"eth0",
		metrics.MetricNetInterfaceRXBytes,
		metrics.MetricNetInterfaceTXBytes,
	)

	result := collector.Collect(ctx, time.Now())
	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
	if result.Message != context.Canceled.Error() {
		t.Fatalf("message = %q, want %q", result.Message, context.Canceled.Error())
	}
	if len(result.Samples) != 0 {
		t.Fatalf("samples length = %d, want 0", len(result.Samples))
	}

	assertReadPaths(t, reader.reads, []string{"/sys/class/net/eth0/statistics/rx_bytes"})
}

func TestCollectors_ContextCancellationUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		collector Collector
	}{
		{
			name:      "cpu collector",
			collector: NewCPUCollector(&panicFileReader{}),
		},
		{
			name:      "memory collector",
			collector: NewMemoryCollector(&panicFileReader{}),
		},
		{
			name: "interface collector",
			collector: NewInterfaceCollector(
				&panicFileReader{},
				"eth0",
				metrics.MetricNetInterfaceRXBytes,
				metrics.MetricNetInterfaceTXBytes,
			),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.collector.Collect(ctx, time.Now())
			if result.Status != CollectStatusUnavailable {
				t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
			}
			if result.Message != context.Canceled.Error() {
				t.Fatalf("message = %q, want %q", result.Message, context.Canceled.Error())
			}
			if len(result.Samples) != 0 {
				t.Fatalf("samples length = %d, want 0", len(result.Samples))
			}
		})
	}
}

func assertInterfaceSample(
	t *testing.T,
	sample metrics.Sample,
	name metrics.MetricName,
	timestamp time.Time,
	value float64,
	iface string,
) {
	t.Helper()

	if sample.Name != name {
		t.Fatalf("sample name = %q, want %q", sample.Name, name)
	}
	if sample.Timestamp != timestamp {
		t.Fatalf("sample timestamp = %s, want %s", sample.Timestamp, timestamp)
	}
	if sample.Value != value {
		t.Fatalf("sample value = %v, want %v", sample.Value, value)
	}
	if sample.Labels["interface"] != iface {
		t.Fatalf("sample interface label = %q, want %q", sample.Labels["interface"], iface)
	}
}

type fakeFileReader struct {
	files map[string][]string
}

func (r *fakeFileReader) ReadFile(path string) ([]byte, error) {
	values, ok := r.files[path]
	if !ok || len(values) == 0 {
		return nil, errors.New("file not found")
	}

	value := values[0]
	if len(values) > 1 {
		r.files[path] = values[1:]
	}

	return []byte(value), nil
}

type sequenceFileReader struct {
	mu       sync.Mutex
	path     string
	call     int
	makeData func(call int) string
}

func (r *sequenceFileReader) ReadFile(path string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if path != r.path {
		return nil, errors.New("file not found")
	}

	r.call++

	return []byte(r.makeData(r.call)), nil
}

type panicFileReader struct{}

func (r *panicFileReader) ReadFile(path string) ([]byte, error) {
	panic("unexpected read for " + path)
}

type cancelAfterReadFileReader struct {
	cancel context.CancelFunc
	files  map[string]string
	reads  []string
}

func (r *cancelAfterReadFileReader) ReadFile(path string) ([]byte, error) {
	r.reads = append(r.reads, path)
	value, ok := r.files[path]
	if !ok {
		return nil, errors.New("file not found")
	}

	r.cancel()

	return []byte(value), nil
}

func assertReadPaths(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("read paths length = %d, want %d; paths = %#v", len(got), len(want), got)
	}
	for i, path := range got {
		if path != want[i] {
			t.Fatalf("read path[%d] = %q, want %q; paths = %#v", i, path, want[i], got)
		}
	}
}
