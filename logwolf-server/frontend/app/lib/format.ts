import type { Severity } from '@logwolf/client-js';

import { locale } from './locale';

export function formatPercent(n: number) {
	return n.toLocaleString(locale, { style: 'percent' });
}

export const severityMap: Record<Severity, string> = {
	info: 'INFO',
	warning: 'WARNING',
	error: 'ERROR',
	critical: 'CRITICAL',
} as const;

export function formatSeverity(s: Severity | null | undefined) {
	if (!s) return '-';
	return severityMap[s];
}
