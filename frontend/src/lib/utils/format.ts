/**
 * Pure timestamp formatting utilities. No Svelte dependencies.
 * Both functions return '—' for empty, null, or unparseable inputs.
 */

const FALLBACK = '—';

/**
 * Converts an ISO 8601 string to a human-readable relative time string.
 * - < 60s  → "just now"
 * - < 60m  → "N minutes ago"
 * - < 24h  → "today at HH:MM"
 * - < 48h  → "yesterday at HH:MM"
 * - older  → locale date + time, e.g. "4 May 2026, 14:30"
 */
export function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) return FALLBACK;
  const date = new Date(iso);
  if (isNaN(date.getTime())) return FALLBACK;

  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);

  if (diffSec < 60) return 'just now';
  if (diffMin < 60) return `${diffMin} minute${diffMin === 1 ? '' : 's'} ago`;

  const timeStr = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });

  if (diffHour < 24) return `today at ${timeStr}`;
  if (diffHour < 48) return `yesterday at ${timeStr}`;

  return date.toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

/**
 * Converts an ISO 8601 string to a full locale datetime string.
 * Suitable for use in title/tooltip attributes.
 * Returns '—' for empty, null, or unparseable inputs.
 */
export function formatAbsoluteTime(iso: string | null | undefined): string {
  if (!iso) return FALLBACK;
  const date = new Date(iso);
  if (isNaN(date.getTime())) return FALLBACK;
  return date.toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}
