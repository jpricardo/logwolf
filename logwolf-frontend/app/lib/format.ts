export function formatPercent(n: number) {
	return n.toLocaleString(navigator.language, { style: 'percent' });
}
