import { describe, it, expect } from 'vitest';
import { seriesStats, formatTooltipTime, formatMetricValue } from './monitoring';

describe('seriesStats', () => {
  it('returns min, max, avg for a populated series', () => {
    const data = [{ value: 10 }, { value: 20 }, { value: 30 }];
    expect(seriesStats(data)).toEqual({ min: 10, max: 30, avg: 20 });
  });

  it('returns zeros for empty series', () => {
    expect(seriesStats([])).toEqual({ min: 0, max: 0, avg: 0 });
  });

  it('handles single-element series', () => {
    expect(seriesStats([{ value: 42 }])).toEqual({ min: 42, max: 42, avg: 42 });
  });
});

describe('formatTooltipTime', () => {
  it('returns HH:MM for 24h window', () => {
    const d = new Date('2026-05-11T14:30:00Z');
    const result = formatTooltipTime(d, '24h');
    // Should contain HH:MM pattern but not a month name
    expect(result).toMatch(/\d{2}:\d{2}/);
    expect(result).not.toMatch(/May|Jan|Feb/);
  });

  it('returns date+time for 7d window', () => {
    const d = new Date('2026-05-11T14:30:00Z');
    const result = formatTooltipTime(d, '7d');
    // Should contain a day number and time
    expect(result.length).toBeGreaterThan(6);
  });

  it('returns date+time for 30d window', () => {
    const d = new Date('2026-05-11T14:30:00Z');
    const result = formatTooltipTime(d, '30d');
    expect(result.length).toBeGreaterThan(6);
  });
});
