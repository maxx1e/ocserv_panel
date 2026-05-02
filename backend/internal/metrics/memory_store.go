package metrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	samples []Sample
	closed  bool
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		samples: []Sample{},
	}
}

func (s *MemoryStore) Append(ctx context.Context, samples []Sample) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cloned := make([]Sample, 0, len(samples))
	for _, sample := range samples {
		if err := ValidateMetric(sample.Name); err != nil {
			return err
		}
		if !isFinite(sample.Value) {
			return fmt.Errorf("%w: non-finite value", ErrInvalidSample)
		}

		cloned = append(cloned, Sample{
			Name:      sample.Name,
			Labels:    cloneLabels(sample.Labels),
			Timestamp: sample.Timestamp,
			Value:     sample.Value,
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrStoreClosed
	}

	s.samples = append(s.samples, cloned...)

	return nil
}

func (s *MemoryStore) QueryRange(ctx context.Context, query Query) (Series, error) {
	if err := ctx.Err(); err != nil {
		return Series{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Series{}, ErrStoreClosed
	}

	if err := ValidateMetric(query.Metric); err != nil {
		return Series{}, err
	}
	if !query.End.After(query.Start) {
		return Series{}, fmt.Errorf("%w: end must be after start", ErrInvalidRange)
	}
	if query.Step <= 0 {
		return Series{}, fmt.Errorf("%w: non-positive step", ErrInvalidStep)
	}

	series := Series{
		Metric: query.Metric,
		Labels: cloneLabels(query.Labels),
		Points: []Point{},
	}

	buckets := map[int64]Point{}
	for _, sample := range s.samples {
		if sample.Name != query.Metric {
			continue
		}
		if !labelsMatch(sample.Labels, query.Labels) {
			continue
		}
		if sample.Timestamp.Before(query.Start) || sample.Timestamp.After(query.End) {
			continue
		}

		bucket := bucketIndex(query.Start, query.Step, sample.Timestamp)
		point := Point{
			Timestamp: sample.Timestamp,
			Value:     sample.Value,
		}

		current, ok := buckets[bucket]
		if !ok || current.Timestamp.Before(sample.Timestamp) {
			buckets[bucket] = point
		}
	}

	for _, point := range buckets {
		series.Points = append(series.Points, point)
	}

	sort.Slice(series.Points, func(i, j int) bool {
		return series.Points[i].Timestamp.Before(series.Points[j].Timestamp)
	})

	return series, nil
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.samples = []Sample{}

	return nil
}

func labelsMatch(sampleLabels map[string]string, queryLabels map[string]string) bool {
	for key, want := range queryLabels {
		got, ok := sampleLabels[key]
		if !ok || got != want {
			return false
		}
	}

	return true
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func bucketIndex(start time.Time, step time.Duration, timestamp time.Time) int64 {
	return int64(timestamp.Sub(start) / step)
}
