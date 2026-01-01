import { locale } from './locale';

export function formatPercent(n: number) {
	return n.toLocaleString(locale, { style: 'percent' });
}
