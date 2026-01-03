import type { Event } from '~/api/events';
import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { events: Event[] };
export function AverageDuration({ className = '', events, ...props }: Props) {
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
