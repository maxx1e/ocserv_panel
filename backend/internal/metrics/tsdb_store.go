package metrics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

type TSDBStore struct {
	mu     sync.RWMutex
	db     *tsdb.DB
	closed bool
}

var _ Store = (*TSDBStore)(nil)

func OpenTSDBStore(path string, retention time.Duration) (*TSDBStore, error) {
	options := tsdb.DefaultOptions()
	options.RetentionDuration = retention.Milliseconds()

	db, err := tsdb.Open(
		path,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
		options,
		tsdb.NewDBStats(),
	)
	if err != nil {
		return nil, fmt.Errorf("open tsdb: %w", err)
	}

	return &TSDBStore{db: db}, nil
}

func (s *TSDBStore) Append(ctx context.Context, samples []Sample) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	for _, sample := range samples {
		if err := ValidateMetric(sample.Name); err != nil {
			return err
		}
		if !isFinite(sample.Value) {
			return fmt.Errorf("%w: non-finite value", ErrInvalidSample)
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrStoreClosed
	}

	appender := s.db.Appender(ctx)
	for _, sample := range samples {
		if err := ctx.Err(); err != nil {
			if rollbackErr := appender.Rollback(); rollbackErr != nil {
				return fmt.Errorf("rollback tsdb append after context error: %w", rollbackErr)
			}

			return err
		}

		_, err := appender.Append(
			0,
			labelsForSample(sample),
			sample.Timestamp.UnixMilli(),
			sample.Value,
		)
		if err != nil {
			if rollbackErr := appender.Rollback(); rollbackErr != nil {
				return fmt.Errorf("rollback tsdb append: %w", rollbackErr)
			}

			return fmt.Errorf("append tsdb sample: %w", err)
		}
	}

	if err := appender.Commit(); err != nil {
		return fmt.Errorf("commit tsdb append: %w", err)
	}

	return nil
}

func (s *TSDBStore) QueryRange(ctx context.Context, query Query) (Series, error) {
	if err := ctx.Err(); err != nil {
		return Series{}, err
	}
	if s == nil {
		return Series{}, ErrStoreClosed
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

	querier, err := s.db.Querier(query.Start.UnixMilli(), query.End.UnixMilli())
	if err != nil {
		return Series{}, fmt.Errorf("open tsdb querier: %w", err)
	}
	defer querier.Close()

	matchers := matchersForQuery(query)
	sets := querier.Select(ctx, false, selectHintsForQuery(query), matchers...)
	series := Series{
		Metric: query.Metric,
		Labels: cloneLabels(query.Labels),
		Points: []Point{},
	}
	buckets := map[int64]Point{}

	for sets.Next() {
		if err := ctx.Err(); err != nil {
			return Series{}, err
		}

		if err := collectSeriesPoints(sets.At(), query, buckets); err != nil {
			return Series{}, err
		}
	}
	if err := sets.Err(); err != nil {
		return Series{}, fmt.Errorf("select tsdb series: %w", err)
	}

	for _, point := range buckets {
		series.Points = append(series.Points, point)
	}
	sort.Slice(series.Points, func(i, j int) bool {
		return series.Points[i].Timestamp.Before(series.Points[j].Timestamp)
	})

	return series, nil
}

func (s *TSDBStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close tsdb: %w", err)
	}

	return nil
}

func (s *TSDBStore) QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	now := time.Now().UTC()
	querier, err := s.db.Querier(now.Add(-time.Hour).UnixMilli(), now.UnixMilli())
	if err != nil {
		return map[string]string{}, nil
	}
	defer querier.Close()

	matchers := []*labels.Matcher{
		labels.MustNewMatcher(labels.MatchEqual, labels.MetricName, string(metric)),
	}
	sets := querier.Select(ctx, false, nil, matchers...)
	for sets.Next() {
		series := sets.At()
		lbls := series.Labels()
		result := make(map[string]string, lbls.Len())
		lbls.Range(func(l labels.Label) {
			if l.Name != labels.MetricName {
				result[l.Name] = l.Value
			}
		})
		return result, nil
	}
	return map[string]string{}, nil
}

func labelsForSample(sample Sample) labels.Labels {
	labelMap := cloneLabels(sample.Labels)
	labelMap[labels.MetricName] = string(sample.Name)

	return labels.FromMap(labelMap)
}

func matchersForQuery(query Query) []*labels.Matcher {
	matchers := make([]*labels.Matcher, 0, len(query.Labels)+1)
	matchers = append(matchers, labels.MustNewMatcher(labels.MatchEqual, labels.MetricName, string(query.Metric)))
	for name, value := range query.Labels {
		matchers = append(matchers, labels.MustNewMatcher(labels.MatchEqual, name, value))
	}

	return matchers
}

func selectHintsForQuery(query Query) *storage.SelectHints {
	return &storage.SelectHints{
		Start: query.Start.UnixMilli(),
		End:   query.End.UnixMilli(),
		Step:  query.Step.Milliseconds(),
	}
}

func collectSeriesPoints(tsdbSeries storage.Series, query Query, buckets map[int64]Point) error {
	iterator := tsdbSeries.Iterator(nil)
	for valueType := iterator.Next(); valueType != chunkenc.ValNone; valueType = iterator.Next() {
		if valueType != chunkenc.ValFloat {
			continue
		}

		timestampMillis, value := iterator.At()
		timestamp := time.UnixMilli(timestampMillis).UTC()
		if timestamp.Before(query.Start) || timestamp.After(query.End) {
			continue
		}

		bucket := bucketIndex(query.Start, query.Step, timestamp)
		point := Point{Timestamp: timestamp, Value: value}
		current, ok := buckets[bucket]
		if !ok || current.Timestamp.Before(timestamp) {
			buckets[bucket] = point
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterate tsdb series: %w", err)
	}

	return nil
}
