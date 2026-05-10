import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

function formatUptime(upSince: string | undefined): string {
  if (!upSince) return '—';
  const date = new Date(upSince);
  if (isNaN(date.getTime())) return '—';
  const ms = Date.now() - date.getTime();
  if (ms < 0) return '—';
  const totalMin = Math.floor(ms / 60000);
  const totalHours = Math.floor(totalMin / 60);
  const totalDays = Math.floor(totalHours / 24);
  if (totalDays > 0) return `${totalDays}d ${totalHours % 24}h`;
  if (totalHours > 0) return `${totalHours}h ${totalMin % 60}m`;
  return `${totalMin}m`;
}

function formatDuration(seconds: number | undefined): string {
  if (!seconds) return '—';
  if (seconds >= 3600) {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
  }
  if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
  return `${seconds}s`;
}

function relativeDate(iso: string | undefined): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (isNaN(date.getTime())) return '—';
  const diffMs = Date.now() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMin / 60);
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays === 0) return 'today';
  if (diffDays < 2) return `${diffDays}d ago`;
  if (diffDays < 30) return `${diffDays}d ago`;
  return `${diffDays}d ago`;
}

describe('formatUptime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-10T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns — for undefined', () => {
    expect(formatUptime(undefined)).toBe('—');
  });

  it('returns — for invalid date string', () => {
    expect(formatUptime('not-a-date')).toBe('—');
  });

  it('formats minutes only when uptime < 1 hour', () => {
    const upSince = new Date('2026-05-10T11:45:00Z').toISOString();
    expect(formatUptime(upSince)).toBe('15m');
  });

  it('formats hours and minutes when uptime < 1 day', () => {
    const upSince = new Date('2026-05-10T08:30:00Z').toISOString();
    expect(formatUptime(upSince)).toBe('3h 30m');
  });

  it('formats days and hours when uptime >= 1 day', () => {
    const upSince = new Date('2026-05-04T07:29:00Z').toISOString();
    const result = formatUptime(upSince);
    expect(result).toBe('6d 4h');
  });
});

describe('formatDuration', () => {
  it('returns — for undefined', () => {
    expect(formatDuration(undefined)).toBe('—');
  });

  it('returns — for zero', () => {
    expect(formatDuration(0)).toBe('—');
  });

  it('formats seconds when < 60', () => {
    expect(formatDuration(45)).toBe('45s');
  });

  it('formats minutes when >= 60 and < 3600', () => {
    expect(formatDuration(150)).toBe('2m');
  });

  it('formats hours and minutes when >= 3600', () => {
    expect(formatDuration(30840)).toBe('8h 34m');
  });

  it('formats exactly 1 hour', () => {
    expect(formatDuration(3600)).toBe('1h 0m');
  });
});

describe('relativeDate', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-10T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns — for undefined', () => {
    expect(relativeDate(undefined)).toBe('—');
  });

  it('returns — for invalid date', () => {
    expect(relativeDate('bad')).toBe('—');
  });

  it('returns today when same day', () => {
    expect(relativeDate('2026-05-10T07:29:00Z')).toBe('today');
  });

  it('returns Xd ago for older dates', () => {
    expect(relativeDate('2026-05-04T07:29:00Z')).toBe('6d ago');
  });
});
