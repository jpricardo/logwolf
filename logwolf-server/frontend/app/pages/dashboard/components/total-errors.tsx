import { TriangleAlert } from 'lucide-react';
import { use } from 'react';

import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';

import { StatCard } from './stat-card';

type Props = { className?: string; p: Promise<Metrics> };
export function TotalErrors({ className, p }: Props) {
	const metrics = use(p);

	return (
		<StatCard
			className={className}
			label='Error events'
			icon={TriangleAlert}
			tone={metrics.total_errors > 0 ? 'error' : 'default'}
			value={metrics.total_errors.toLocaleString(locale)}
			footer={
				<>
					Including{' '}
					<span suppressHydrationWarning className='font-medium text-foreground tabular-nums'>
						{metrics.total_critical.toLocaleString(locale)}
					</span>{' '}
					critical
				</>
			}
		/>
	);
}

export function TotalErrorsSkeleton({ className }: { className?: string }) {
	return <StatCard className={className} label='Error events' icon={TriangleAlert} footer />;
}
