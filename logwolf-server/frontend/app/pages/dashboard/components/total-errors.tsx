import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import type { Log } from '~/lib/api';
import { formatPercent } from '~/lib/format';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { logs: Log[] };
export function TotalErrors({ className = '', logs, ...props }: Props) {
	const errors = logs.filter((l) => l.severity === 'error');

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error logs</CardDescription>
				<CardTitle className='text-3xl'>{errors.length}</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>
					~{formatPercent(errors.length / logs.length)} of the logs are errors!
				</span>
			</CardFooter>
		</Card>
	);
}
