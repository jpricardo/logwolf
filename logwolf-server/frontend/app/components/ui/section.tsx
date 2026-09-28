import { cn } from '~/lib/utils';

type Props = Omit<React.ComponentProps<'section'>, 'title'> & {
	title?: React.ReactNode;
	description?: React.ReactNode;
	addon?: React.ReactNode;
};

export function Section({ title, description, addon, className = '', children, ...props }: Props) {
	return (
		<section className={cn('flex flex-col gap-3', className)} {...props}>
			{(!!title || !!addon) && (
				<div className='flex flex-row items-end justify-between gap-4'>
					<div className='flex flex-col gap-1'>
						{title && <h2 className='text-sm font-semibold tracking-tight'>{title}</h2>}
						{description && <p className='text-sm text-muted-foreground'>{description}</p>}
					</div>
					{addon && <div className='shrink-0'>{addon}</div>}
				</div>
			)}
			<div>{children}</div>
		</section>
	);
}
