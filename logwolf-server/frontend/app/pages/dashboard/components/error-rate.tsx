import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { locale } from '~/lib/locale';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { timespan: number; events: LogwolfEventData[] };
export function ErrorRate({ className = '', timespan, events, ...props }: Props) {
	const minutes = timespan / (1000 * 60);
	const hours = minutes / 60;

	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error rate</CardDescription>
				<CardTitle className='text-3xl'>
					~{(events.length / minutes).toLocaleString(locale, { maximumFractionDigits: 2 })} TPM
				</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>In the last {hours} hours</span>
			</CardFooter>
		</Card>
	);
}
