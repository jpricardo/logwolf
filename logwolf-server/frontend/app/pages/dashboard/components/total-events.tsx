import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';
import { use } from 'react';

import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<LogwolfEventData[]> };
export function TotalEvents({ className = '', p, ...props }: Props) {
	const events = use(p);

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Total events</CardDescription>
				<CardTitle className='text-3xl'>{events.length}</CardTitle>
			</CardHeader>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function TotalEventsSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Total events</CardDescription>
				<CardTitle>
					<Skeleton className='h-10 w-full' />
				</CardTitle>
			</CardHeader>
		</Card>
	);
}
