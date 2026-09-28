import { cn } from '~/lib/utils';

type Props<T> = Omit<React.ComponentProps<'div'>, 'children'> & {
	label: React.ReactNode;
	value: T;
};

export function InfoItem<T extends React.ReactNode>({ label, value, className, ...props }: Props<T>) {
	return (
		<div className={cn('flex flex-col gap-1.5 px-5 py-3.5', className)} {...props}>
			<dt className='text-xs text-muted-foreground'>{label}</dt>
			<dd className='text-sm'>{value}</dd>
		</div>
	);
}
