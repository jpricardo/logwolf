import { cn } from '~/lib/utils';

type Props = Omit<React.ComponentProps<'section'>, 'title'> & {
	title: string;
	description: React.ReactNode;
	tone?: 'default' | 'destructive';
};

/** One settings topic: what it is on the left, its controls on the right. */
export function SettingsRow({ title, description, tone = 'default', className, children, ...props }: Props) {
	return (
		<section
			className={cn(
				'grid grid-cols-1 gap-4 border-t pt-8 first:border-t-0 first:pt-0 md:grid-cols-[15rem_minmax(0,1fr)] md:gap-10',
				className,
			)}
			{...props}
		>
			<div className='flex flex-col gap-1'>
				<h2 className={cn('text-sm font-semibold tracking-tight', tone === 'destructive' && 'text-destructive')}>
					{title}
				</h2>
				<p className='text-sm text-muted-foreground'>{description}</p>
			</div>

			<div className='flex min-w-0 flex-col gap-3'>{children}</div>
		</section>
	);
}

/** The strip under a settings form: a hint on the left, its submit button on the right. */
export function SettingsFooter({ hint, children }: { hint?: React.ReactNode; children?: React.ReactNode }) {
	return (
		<div className='flex min-h-14 flex-row items-center justify-between gap-4 border-t bg-muted/40 px-5 py-3'>
			<p className='text-xs text-muted-foreground'>{hint}</p>
			{children}
		</div>
	);
}
