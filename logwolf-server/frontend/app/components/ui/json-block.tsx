import { Card, CardContent } from '~/components/ui/card';
import { cn } from '~/lib/utils';

export type Props = Omit<React.ComponentProps<typeof Card>, 'children'> & {
	data: Record<string | number | symbol, unknown>;
};

export function JSONBlock({ data, className, ...props }: Props) {
	return (
		<Card className={cn('shadow-none overflow-x-auto', className)} {...props}>
			<CardContent className='max-w-0'>
				<pre className='font-mono'>
					<code>{JSON.stringify(data, undefined, '\t')}</code>
				</pre>
			</CardContent>
		</Card>
	);
}
