import { use } from 'react';
import { Bar, BarChart, LabelList, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '~/components/ui/chart';
import type { Metrics } from '~/lib/api';
import { cn } from '~/lib/utils';

const maxBarAmt = 5;

type Props = React.ComponentProps<typeof Card> & { p: Promise<Metrics> };
export function TagsBarChart({ className, p, ...props }: Props) {
	const metrics = use(p);
	const data = metrics.top_tags?.toSorted((a, b) => b.count - a.count).slice(0, maxBarAmt);

	return (
		<Card className={cn('shadow-none h-full', className)} {...props}>
			<CardHeader>
				<CardDescription>Tags</CardDescription>
				<CardTitle>Most frequent tags</CardTitle>
				<CardContent className='flex-1 pb-0'>
					<ChartContainer config={{}}>
						<BarChart layout='vertical' accessibilityLayer data={data}>
							<XAxis type='number' dataKey='ammount' hide />
							<YAxis type='category' dataKey='tag' tickLine={false} tickMargin={10} axisLine={false} />
							<ChartTooltip cursor={false} content={<ChartTooltipContent indicator='line' />} />
							<Bar dataKey='ammount' radius={4}>
								<LabelList dataKey='ammount' position='right' offset={8} className='text-foreground' fontSize={12} />
							</Bar>
							<LabelList />
						</BarChart>
					</ChartContainer>
				</CardContent>
			</CardHeader>
		</Card>
	);
}

type SkeletonProps = React.ComponentProps<typeof Card>;
export function TagsBarChartSkeleton({ className, ...props }: SkeletonProps) {
	return (
		<Card className={cn('shadow-none h-full', className)} {...props}>
			<CardHeader>
				<CardDescription>Tags</CardDescription>
				<CardTitle>Most frequent tags</CardTitle>
				<CardContent className='flex-1 pb-0'></CardContent>
			</CardHeader>
		</Card>
	);
}
