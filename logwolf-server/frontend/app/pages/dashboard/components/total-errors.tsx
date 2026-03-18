import { use } from 'react';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import type { Metrics } from '~/lib/api';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<Metrics> };
export function TotalErrors({ className = '', p, ...props }: Props) {
	const metrics = use(p);

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error events</CardDescription>
				<CardTitle className='text-3xl'>{metrics.total_errors}</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>
					Including <span className='font-bold'>{metrics.total_critical}</span> critical events!
				</span>
			</CardFooter>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function TotalErrorsSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error events</CardDescription>
				<CardTitle>
					<Skeleton className='h-10 w-full' />
				</CardTitle>
			</CardHeader>
		</Card>
	);
}
