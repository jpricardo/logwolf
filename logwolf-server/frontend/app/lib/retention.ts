/**
 * Whether moving a project from `from` days of retention to `to` shortens it.
 * 0 is forever, the longest of all.
 *
 * Mirrors the broker's `data.LowersRetention`: any member may keep logs longer,
 * but lowering retention makes the next cleanup pass delete everything outside
 * the new window, so only an owner may.
 */
export function lowersRetention(from: number, to: number): boolean {
	if (to === 0) return false;
	return from === 0 || to < from;
}
