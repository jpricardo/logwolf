import { Card, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import type { Log } from '~/lib/api';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { logs: Log[] };
export function TotalLogs({ className = '', logs, ...props }: Props) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Total logs</CardDescription>
				<CardTitle className='text-3xl'>{logs.length}</CardTitle>
			</CardHeader>
		</Card>
	);
}
