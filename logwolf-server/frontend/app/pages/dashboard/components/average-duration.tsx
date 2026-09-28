import { Timer } from 'lucide-react';
import { use } from 'react';

import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';

import { StatCard } from './stat-card';

type Props = { className?: string; p: Promise<Metrics> };
export function AverageDuration({ className, p }: Props) {
	const metrics = use(p);
	const hasDuration = metrics.avg_duration_ms > 0;

	return (
		<StatCard
			className={className}
			label='Average duration'
			icon={Timer}
			value={hasDuration ? metrics.avg_duration_ms.toLocaleString(locale, { maximumFractionDigits: 2 }) : '—'}
			unit={hasDuration ? 'ms' : undefined}
			footer='Of events that recorded one'
		/>
	);
}

export function AverageDurationSkeleton({ className }: { className?: string }) {
	return <StatCard className={className} label='Average duration' icon={Timer} footer />;
}
