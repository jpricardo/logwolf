import type { LucideIcon } from 'lucide-react';

import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { cn } from '~/lib/utils';

type Props = Omit<React.ComponentProps<typeof Card>, 'children'> & {
	label: string;
	icon: LucideIcon;
	/** Undefined renders the tile loading. */
	value?: React.ReactNode;
	unit?: string;
	footer?: React.ReactNode;
	tone?: 'default' | 'error';
};

export function StatCard({ label, icon: Icon, value, unit, footer, tone = 'default', className, ...props }: Props) {
	return (
		<Card className={cn('gap-3 px-5', className)} {...props}>
			<div className='flex items-center justify-between gap-2'>
				<span className='text-sm text-muted-foreground'>{label}</span>
				<span
					className={cn(
						'flex size-7 items-center justify-center rounded-md border bg-muted/60 text-muted-foreground',
						tone === 'error' && 'border-severity-error/25 bg-severity-error/10 text-severity-error',
					)}
				>
					<Icon className='size-3.5' />
				</span>
			</div>

			{value === undefined ? (
				<Skeleton className='h-9 w-2/3' />
			) : (
				<div className='flex items-baseline gap-1.5'>
					{/* Numbers are formatted in the browser's locale, which the server can't know. */}
					<span suppressHydrationWarning className='text-3xl font-semibold tracking-tight tabular-nums'>
						{value}
					</span>
					{unit && <span className='text-sm text-muted-foreground'>{unit}</span>}
				</div>
			)}

			{footer !== undefined && (
				<div suppressHydrationWarning className='text-xs text-muted-foreground'>
					{value === undefined ? <Skeleton className='h-4 w-1/2' /> : footer}
				</div>
			)}
		</Card>
	);
}
