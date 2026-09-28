import { Activity } from 'lucide-react';
import { use } from 'react';

import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';

import { StatCard } from './stat-card';

type Props = { className?: string; p: Promise<Metrics> };
export function EventRate({ className, p }: Props) {
	const metrics = use(p);

	const minutes = 24 * 60;
	const perMinute = metrics.events_last_24h / minutes;

	return (
		<StatCard
			className={className}
			label='Event rate'
			icon={Activity}
			value={perMinute.toLocaleString(locale, { maximumFractionDigits: 2 })}
			unit='/ min'
			footer={`${metrics.events_last_24h.toLocaleString(locale)} in the last 24 hours`}
		/>
	);
}

export function EventRateSkeleton({ className }: { className?: string }) {
	return <StatCard className={className} label='Event rate' icon={Activity} footer />;
}
