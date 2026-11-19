/**
 * Cache lifetime (`gcTime`) for a query, applied in the browser only.
 *
 * On the server every page render builds its own query client. A finite
 * `gcTime` there starts a timer per query, and until it fires that timer keeps
 * the query, its client and what they reference from the render in memory.
 * The exchange rates, which every page header reads, asked for 24 hours, so
 * each rendered page stayed in the server's memory for a day.
 *
 * `Infinity` is the query library's own default on the server. It starts no
 * timer, and a render's memory is freed with its request.
 */
export function browserGcTime(milliseconds: number): number {
  return typeof window === 'undefined' ? Infinity : milliseconds;
}
