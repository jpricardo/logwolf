import { Badge } from '~/components/ui/badge';
import { cn } from '~/lib/utils';

type Props = { tags: string[]; className?: string };
export function TagList({ tags, className }: Props) {
	if (tags.length === 0) return <span className='text-muted-foreground'>—</span>;

	return (
		<div className={cn('flex flex-row flex-wrap items-center gap-1', className)}>
			{tags
				.toSorted((a, b) => a.localeCompare(b))
				.map((t) => (
					<Badge key={t} variant={t === 'error' ? 'destructive' : 'secondary'} className='font-mono font-normal'>
						{t}
					</Badge>
				))}
		</div>
	);
}
