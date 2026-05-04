/**
 * Pure client-side pagination utility. No Svelte dependencies.
 */

export type PaginationResult<T> = {
  items: T[];
  totalPages: number;
  page: number;      // 1-indexed, clamped to [1, totalPages]
  pageSize: number;
};

/**
 * Slices an array for the given 1-indexed page.
 *
 * - `page` is clamped to [1, totalPages]
 * - `totalPages` is always at least 1, even for an empty array
 * - Returns an empty `items` array (not an error) when the input is empty
 */
export function paginate<T>(
  items: T[],
  page: number,
  pageSize: number
): PaginationResult<T> {
  const safePageSize = Math.max(1, pageSize);
  const totalPages = Math.max(1, Math.ceil(items.length / safePageSize));
  const clampedPage = Math.min(Math.max(1, page), totalPages);
  const start = (clampedPage - 1) * safePageSize;
  const end = start + safePageSize;
  return {
    items: items.slice(start, end),
    totalPages,
    page: clampedPage,
    pageSize: safePageSize
  };
}
