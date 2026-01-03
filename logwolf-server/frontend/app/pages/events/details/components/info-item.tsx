import { cn } from '~/lib/utils';

type Props<T> = Omit<React.ComponentProps<'div'>, 'children'> & {
	label: React.ReactNode;
	value: T;
};

export function InfoItem<T extends React.ReactNode>({ label, value, className, ...props }: Props<T>) {
	return (
		<div className={cn('flex flex-row gap-4 justify-start items-center', className)} {...props}>
			<span className='text-muted-foreground w-22 text-sm'>{label}</span>
			<span className='text-sm'>{value}</span>
		</div>
	);
}
