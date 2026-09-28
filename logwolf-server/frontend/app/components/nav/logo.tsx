import { cn } from '~/lib/utils';

/** The mark: a few log lines on the primary colour. */
export function LogoMark({ className, ...props }: React.ComponentProps<'svg'>) {
	return (
		<svg viewBox='0 0 24 24' aria-hidden className={cn('size-6 shrink-0', className)} {...props}>
			<rect width='24' height='24' rx='4' className='fill-primary' />
			<rect x='5' y='6.5' width='14' height='2' rx='0.5' className='fill-primary-foreground' />
			<rect x='5' y='11' width='9' height='2' rx='0.5' className='fill-primary-foreground/80' />
			<rect x='5' y='15.5' width='11.5' height='2' rx='0.5' className='fill-primary-foreground/60' />
		</svg>
	);
}

type Props = React.ComponentProps<'span'> & { markClassName?: string };
export function Logo({ className, markClassName, ...props }: Props) {
	return (
		<span className={cn('inline-flex items-center gap-2 font-semibold tracking-tight', className)} {...props}>
			<LogoMark className={markClassName} />
			Logwolf
		</span>
	);
}
