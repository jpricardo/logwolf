import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import type { Log } from '~/lib/api';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { logs: Log[] };
export function AverageDuration({ className = '', logs, ...props }: Props) {
	const total = logs.reduce((acc, l) => acc + (l.data.duration ?? 0), 0);
	const avg = total > 0 ? total / logs.length : '-';

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Average duration</CardDescription>
				<CardTitle className='text-3xl'>{avg}</CardTitle>
			</CardHeader>
		</Card>
	);
}
