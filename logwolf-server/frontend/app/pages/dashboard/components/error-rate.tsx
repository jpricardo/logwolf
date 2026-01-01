import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import type { Log } from '~/lib/api';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

const hours = 24;
const minutes = hours * 60;
const seconds = minutes * 60;
const ms = seconds * 1000;

type Props = React.ComponentProps<typeof Card> & { logs: Log[] };
export function ErrorRate({ className = '', logs, ...props }: Props) {
	const end = new Date().getTime();
	const start = new Date().setTime(end - ms);

	const data = logs.filter((l) => {
		const time = new Date(l.created_at).getTime();
		return l.severity === 'error' && time >= start && time <= end;
	});

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error rate</CardDescription>
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
