import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';

import { Card, CardDescription, CardFooter, CardHeader, CardTitle } from '~/components/ui/card';
import { formatPercent } from '~/lib/format';
import { cn } from '~/lib/utils';

type Props = React.ComponentProps<typeof Card> & { totalEvents: number; errors: LogwolfEventData[] };
export function TotalErrors({ className = '', totalEvents, errors, ...props }: Props) {
	return (
		<Card className={cn('shadow-none', className)} {...props}>
			<CardHeader>
				<CardDescription>Error events</CardDescription>
				<CardTitle className='text-3xl'>{errors.length}</CardTitle>
			</CardHeader>

			<CardFooter>
				<span className='text-muted-foreground'>
					~{formatPercent(errors.length / totalEvents)} of captured events are errors!
				</span>
			</CardFooter>
		</Card>
	);
}
