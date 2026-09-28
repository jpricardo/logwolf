import { cva, type VariantProps } from 'class-variance-authority';

import { formatSeverity } from '~/lib/format';
import { cn } from '~/lib/utils';

const variants = cva(
	'inline-flex w-fit shrink-0 items-center gap-1.5 rounded-sm border px-1.5 py-0.5 font-mono text-[11px] leading-none font-medium tracking-wide whitespace-nowrap',
	{
		variants: {
			variant: {
				info: 'border-severity-info/25 bg-severity-info/10 text-severity-info',
				warning: 'border-severity-warning/30 bg-severity-warning/10 text-severity-warning',
				error: 'border-severity-error/30 bg-severity-error/10 text-severity-error',
				critical: 'border-severity-critical bg-severity-critical text-white',
			},
		},
	},
);

type Props = VariantProps<typeof variants> & { className?: string };
export function SeverityBadge({ variant, className }: Props) {
	return (
		<span data-slot='severity-badge' className={cn(variants({ variant }), className)}>
			<span className='size-1.5 rounded-[1px] bg-current' aria-hidden />
			{formatSeverity(variant)}
		</span>
	);
}
