import type { Severity } from '~/api/events';
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

export function formatSeverity(s: Severity) {
	return severityMap[s];
}
