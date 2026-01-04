import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';
import { use } from 'react';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<LogwolfEventData[]> };
export function TotalErrors({ className = '', p, ...props }: Props) {
	const errors = use(p);
	const critical = errors.filter((e) => e.severity === 'critical');

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error events</CardDescription>
				<CardTitle className='text-3xl'>{errors.length}</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>
					Including <span className='font-bold'>{critical.length}</span> critical events!
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
