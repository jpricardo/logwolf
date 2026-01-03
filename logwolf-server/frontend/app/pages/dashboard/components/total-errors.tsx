import type { Event } from '~/api/events';
import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { formatPercent } from '~/lib/format';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { events: Event[] };
export function TotalErrors({ className = '', events, ...props }: Props) {
	const errors = events.filter((l) => l.severity === 'error' || l.severity === 'critical');

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error events</CardDescription>
				<CardTitle className='text-3xl'>{errors.length}</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>
					~{formatPercent(errors.length / events.length)} of captured events are errors!
				</span>
			</CardFooter>
		</Card>
	);
}
