import { Flame } from 'lucide-react';
import { use } from 'react';

import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';

import { StatCard } from './stat-card';

type Props = { className?: string; p: Promise<Metrics> };
export function ErrorRate({ className, p }: Props) {
	const metrics = use(p);

	const minutes = 24 * 60;
	const perMinute = metrics.errors_last_24h / minutes;

	return (
		<StatCard
			className={className}
			label='Error rate'
			icon={Flame}
			tone={metrics.errors_last_24h > 0 ? 'error' : 'default'}
			value={perMinute.toLocaleString(locale, { maximumFractionDigits: 2 })}
			unit='/ min'
			footer={`${metrics.errors_last_24h.toLocaleString(locale)} in the last 24 hours`}
		/>
	);
}

export function ErrorRateSkeleton({ className }: { className?: string }) {
	return <StatCard className={className} label='Error rate' icon={Flame} footer />;
}
