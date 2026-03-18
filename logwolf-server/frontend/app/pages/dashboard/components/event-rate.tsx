import { use } from 'react';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import type { Metrics } from '~/lib/api';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { p: Promise<Metrics> };
export function EventRate({ className = '', p, ...props }: Props) {
	const metrics = use(p);

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Event rate</CardDescription>
				<CardTitle className='text-3xl'>
					~{metrics.events_last_24h.toLocaleString(locale, { maximumFractionDigits: 2 })} TPM
				</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>In the last 24 hours</span>
			</CardFooter>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function EventRateSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Event rate</CardDescription>
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
