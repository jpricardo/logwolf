import { use } from 'react';

import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<Metrics> };
export function AverageDuration({ className = '', p, ...props }: Props) {
	const metrics = use(p);

	const avg =
		metrics.avg_duration_ms > 0
			? `~${metrics.avg_duration_ms.toLocaleString(locale, { maximumFractionDigits: 2 })}ms`
			: '-';

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Average duration</CardDescription>
				<CardTitle className='text-3xl'>{avg}</CardTitle>
			</CardHeader>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function AverageDurationSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Average duration</CardDescription>
				<CardTitle>
					<Skeleton className='h-10 w-full' />
				</CardTitle>
			</CardHeader>
		</Card>
	);
}
