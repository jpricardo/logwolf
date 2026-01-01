import { cn } from '~/lib/utils';

type Props = React.ComponentProps<'div'> & {
	title?: React.ReactNode;
	addon?: React.ReactNode;
};

export function Section({ title, addon, className = '', children, ...props }: Props) {
	return (
		<section className={cn('flex flex-col gap-2', className)} {...props}>
			{(!!title || !!addon) && (
				<div className='flex flex-row items-center justify-between'>
					<div className='text-muted-foreground'>{title}</div>
					<div>{addon}</div>
				</div>
			)}
			<div>{children}</div>
		</section>
	);
}
