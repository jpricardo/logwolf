import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';
import { use } from 'react';

import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<LogwolfEventData[]> };
export function AverageDuration({ className = '', p, ...props }: Props) {
	const events = use(p);

	const eventsWithDuration = events.filter((e) => e.duration !== undefined);
	const total = eventsWithDuration.reduce((acc, l) => acc + l.duration!, 0);
	const avg =
		total > 0 ? `~${(total / eventsWithDuration.length).toLocaleString(locale, { maximumFractionDigits: 2 })}ms` : '-';

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
