import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';
import { use } from 'react';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { timespan: number; p: Promise<LogwolfEventData[]> };
export function ErrorRate({ className = '', timespan, p, ...props }: Props) {
	const events = use(p);

	const minutes = timespan / (1000 * 60);
	const hours = minutes / 60;

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error rate</CardDescription>
				<CardTitle className='text-3xl'>
					~{(events.length / minutes).toLocaleString(locale, { maximumFractionDigits: 2 })} TPM
				</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>In the last {hours} hours</span>
			</CardFooter>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function ErrorRateSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error rate</CardDescription>
				<CardTitle>
					<Skeleton className='h-10 w-full' />
				</CardTitle>
			</CardHeader>

			<CardFooter>
				<Skeleton className='h-4 w-full' />
			</CardFooter>
		</Card>
	);
}
