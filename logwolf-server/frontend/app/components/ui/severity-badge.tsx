import { cva, type VariantProps } from 'class-variance-authority';

import { formatSeverity } from '~/lib/format';
import { cn } from '~/lib/utils';

import { Badge } from './badge';

const variants = cva('border-transparent rounded-xs', {
	variants: {
		variant: {
			info: 'bg-blue-400 dark:bg-blue-400/50',
			warning: 'bg-yellow-500 dark:bg-yellow-500/90',
			error: 'bg-destructive dark:bg-destructive/50',
			critical: 'font-bold bg-red-600/10 border-red-600 text-red-600 dark:border-red-600 dark:text-red-600',
		},
	},
});

type Props = VariantProps<typeof variants>;
export function SeverityBadge({ variant }: Props) {
	return <Badge className={cn(variants({ variant }))}>{formatSeverity(variant)}</Badge>;
}
