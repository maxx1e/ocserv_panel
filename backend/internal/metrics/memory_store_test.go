package metrics

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestValidateWindowAndStep(t *testing.T) {
	tests := []struct {
		name      string
		window    string
		step      string
		wantError bool
	}{
		{name: "short dashboard window", window: "1h", step: "60s"},
		{name: "long history window", window: "7d", step: "1h"},
		{name: "invalid window", window: "2h", step: "60s", wantError: true},
		{name: "invalid step", window: "1h", step: "30s", wantError: true},
		{name: "too many points", window: "30d", step: "15s", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ValidateWindowAndStep(tt.window, tt.step)
			if tt.wantError && err == nil {
				t.Fatal("ValidateWindowAndStep succeeded, want error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("ValidateWindowAndStep returned error: %v", err)
			}
		})
	}
}

func TestMetricCatalogMarksOcservOnlyUnavailable(t *testing.T) {
	catalog := DefaultCatalog()
	for _, name := range []MetricName{
		MetricUserRXBytes,
		MetricUserTXBytes,
		MetricActiveSessions,
		MetricConnectionRate,
		MetricConcurrentSessions,
		MetricConcurrentSessionsLimit,
	} {
		metric, ok := catalog.Find(name)
		if !ok {
			t.Fatalf("catalog missing %s", name)
		}
		if metric.State != StateUnavailable || metric.Reason == "" {
			t.Fatalf(
				"metric %s state = %s reason = %q, want unavailable with reason",
				name,
				metric.State,
				metric.Reason,
			)
		}
	}
}

func TestMemoryStoreAppendAndQueryRange(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	err := store.Append(ctx, []Sample{
		{
			Name:      MetricHostCPUUtilization,
			Labels:    map[string]string{"source": "host"},
			Timestamp: base,
			Value:     0.25,
		},
		{
			Name:      MetricHostCPUUtilization,
			Labels:    map[string]string{"source": "host"},
			Timestamp: base.Add(time.Minute),
			Value:     0.50,
		},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	series, err := store.QueryRange(ctx, Query{
		Metric: MetricHostCPUUtilization,
		Start:  base.Add(-time.Second),
		End:    base.Add(2 * time.Minute),
		Step:   time.Minute,
	})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 2 {
		t.Fatalf("points = %d, want 2: %#v", len(series.Points), series.Points)
	}
	if series.Points[0].Value != 0.25 || series.Points[1].Value != 0.50 {
		t.Fatalf("points = %#v, want appended values", series.Points)
	}
}

func TestMemoryStoreQueryRangeBucketsSamplesByStep(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	err := store.Append(ctx, []Sample{
		{Name: MetricHostCPUUtilization, Timestamp: base.Add(10 * time.Second), Value: 0.25},
		{Name: MetricHostCPUUtilization, Timestamp: base.Add(50 * time.Second), Value: 0.50},
		{Name: MetricHostCPUUtilization, Timestamp: base.Add(70 * time.Second), Value: 0.75},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	series, err := store.QueryRange(ctx, Query{
		Metric: MetricHostCPUUtilization,
		Start:  base,
		End:    base.Add(2 * time.Minute),
		Step:   time.Minute,
	})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 2 {
		t.Fatalf("points = %d, want 2: %#v", len(series.Points), series.Points)
	}
	if series.Points[0].Timestamp != base.Add(50*time.Second) || series.Points[0].Value != 0.50 {
		t.Fatalf("first point = %#v, want most recent sample in first bucket", series.Points[0])
	}
	if series.Points[1].Timestamp != base.Add(70*time.Second) || series.Points[1].Value != 0.75 {
		t.Fatalf("second point = %#v, want sample in second bucket", series.Points[1])
	}
}

func TestMemoryStoreQueryRangeRejectsNonPositiveStep(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	_, err := store.QueryRange(ctx, Query{
		Metric: MetricHostCPUUtilization,
		Start:  base,
		End:    base.Add(time.Minute),
	})
	if !errors.Is(err, ErrInvalidStep) {
		t.Fatalf("QueryRange error = %v, want %v", err, ErrInvalidStep)
	}
}

func TestMemoryStoreQueryRangeRejectsInvalidTimeRange(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{name: "end before start", start: base, end: base.Add(-time.Second)},
		{name: "end equals start", start: base, end: base},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.QueryRange(ctx, Query{
				Metric: MetricHostCPUUtilization,
				Start:  tt.start,
				End:    tt.end,
				Step:   time.Minute,
			})
			if !errors.Is(err, ErrInvalidRange) {
				t.Fatalf("QueryRange error = %v, want %v", err, ErrInvalidRange)
			}
		})
	}
}

func TestMemoryStoreReturnsErrorAfterClose(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := store.Append(ctx, []Sample{{Name: MetricHostCPUUtilization, Value: 0.25}})
	if !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("Append error = %v, want %v", err, ErrStoreClosed)
	}

	_, err = store.QueryRange(ctx, Query{Metric: MetricHostCPUUtilization})
	if !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("QueryRange error = %v, want %v", err, ErrStoreClosed)
	}
}

func TestMemoryStoreRejectsNonFiniteSampleValues(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value float64
	}{
		{name: "NaN", value: math.NaN()},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "negative infinity", value: math.Inf(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			err := store.Append(ctx, []Sample{
				{
					Name:      MetricHostCPUUtilization,
					Timestamp: base,
					Value:     tt.value,
				},
			})
			if !errors.Is(err, ErrInvalidSample) {
				t.Fatalf("Append error = %v, want %v", err, ErrInvalidSample)
			}

			series, err := store.QueryRange(ctx, Query{
				Metric: MetricHostCPUUtilization,
				Start:  base.Add(-time.Second),
				End:    base.Add(time.Second),
				Step:   time.Second,
			})
			if err != nil {
				t.Fatalf("QueryRange: %v", err)
			}
			if len(series.Points) != 0 {
				t.Fatalf("points = %#v, want no stored samples", series.Points)
			}
		})
	}
}
