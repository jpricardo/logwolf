import type { Event } from '~/api/events';
import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { events: Event[] };
export function TotalEvents({ className = '', events, ...props }: Props) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Total events</CardDescription>
				<CardTitle className='text-3xl'>{events.length}</CardTitle>
			</CardHeader>
		</Card>
	);
}
