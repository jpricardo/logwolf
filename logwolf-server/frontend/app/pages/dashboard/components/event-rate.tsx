import type { Event } from '~/api/events';
import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

const hours = 24;
const minutes = hours * 60;
const seconds = minutes * 60;
const ms = seconds * 1000;

type Props = React.ComponentProps<typeof Card> & { events: Event[] };
export function EventRate({ className = '', events, ...props }: Props) {
	const end = new Date().getTime();
	const start = new Date().setTime(end - ms);

	const data = events.filter((l) => {
		const time = l.created_at.getTime();
		return time >= start && time <= end;
	});

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Event rate</CardDescription>
				<CardTitle className='text-3xl'>
					~{(data.length / minutes).toLocaleString(locale, { maximumFractionDigits: 2 })} TPM
				</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>In the last 24 hours</span>
			</CardFooter>
		</Card>
	);
}
