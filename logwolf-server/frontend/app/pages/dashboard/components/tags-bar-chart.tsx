import { Tags } from 'lucide-react';
import { use } from 'react';
import { Bar, BarChart, LabelList, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '~/components/ui/chart';
import { Skeleton } from '~/components/ui/skeleton';
import type { Metrics } from '~/lib/api';
import { cn } from '~/lib/utils';

const maxBarAmt = 5;

const chartConfig = {
	count: { label: 'Events', color: 'var(--chart-1)' },
} satisfies ChartConfig;

function TagsCardHeader() {
	return (
		<CardHeader>
			<CardTitle>Most frequent tags</CardTitle>
			<CardDescription>Top {maxBarAmt} tags by event count</CardDescription>
		</CardHeader>
	);
}

type Props = React.ComponentProps<typeof Card> & { p: Promise<Metrics> };
export function TagsBarChart({ className, p, ...props }: Props) {
	const metrics = use(p);
	const data = metrics.top_tags?.toSorted((a, b) => b.count - a.count).slice(0, maxBarAmt) ?? [];

	return (
		<Card className={cn('h-full', className)} {...props}>
			<TagsCardHeader />

			<CardContent className='flex flex-1 flex-col justify-center'>
				{data.length === 0 ? (
					<div className='flex flex-1 flex-col items-center justify-center gap-2 rounded-md border border-dashed py-10 text-center'>
						<Tags className='size-5 text-muted-foreground' />
						<p className='text-sm text-muted-foreground'>No tagged events yet.</p>
					</div>
				) : (
					<ChartContainer config={chartConfig} className='aspect-auto h-64 w-full'>
						<BarChart
							layout='vertical'
							accessibilityLayer
							data={data}
							margin={{ left: 0, right: 40 }}
							barCategoryGap={10}
						>
							<XAxis type='number' dataKey='count' hide />
							<YAxis
								type='category'
								dataKey='tag'
								tickLine={false}
								axisLine={false}
								tickMargin={8}
								width={96}
								className='font-mono'
							/>
							<ChartTooltip cursor={false} content={<ChartTooltipContent indicator='line' />} />
							<Bar dataKey='count' fill='var(--color-count)' radius={2}>
								<LabelList
									dataKey='count'
									position='right'
									offset={8}
									className='fill-foreground font-mono tabular-nums'
									fontSize={12}
								/>
							</Bar>
						</BarChart>
					</ChartContainer>
				)}
			</CardContent>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function TagsBarChartSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('h-full', className)} {...props}>
			<TagsCardHeader />

			<CardContent className='flex flex-1 flex-col justify-center gap-3'>
				{[90, 70, 55, 40, 25].map((w) => (
					<Skeleton key={w} className='h-7' style={{ width: `${w}%` }} />
				))}
			</CardContent>
		</Card>
	);
}
