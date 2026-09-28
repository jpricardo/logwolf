import { Layers } from 'lucide-react';
import { use } from 'react';

import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';

import { StatCard } from './stat-card';

type Props = { className?: string; p: Promise<Metrics> };
export function TotalEvents({ className, p }: Props) {
	const metrics = use(p);

	return (
		<StatCard
			className={className}
			label='Total events'
			icon={Layers}
			value={metrics.total_events.toLocaleString(locale)}
			footer='Across the retention window'
		/>
	);
}

export function TotalEventsSkeleton({ className }: { className?: string }) {
	return <StatCard className={className} label='Total events' icon={Layers} footer />;
}
