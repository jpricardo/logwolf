import type { LogwolfEventData } from '@jpricardo/logwolf-client-js';
import { Bar, BarChart, LabelList, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '~/components/ui/chart';

const maxBarAmt = 5;

type Props = { events: LogwolfEventData[] };
export function TagsBarChart({ events }: Props) {
	const tags = Array.from(new Set(events.flatMap((l) => l.tags)));

	const data = tags
		.reduce<{ tag: string; ammount: number }[]>((acc, curr) => {
			const amt = events.filter((l) => l.tags.includes(curr)).length;
			return [...acc, { tag: curr, ammount: amt }];
		}, [])
		.toSorted((a, b) => b.ammount - a.ammount)
		.slice(0, maxBarAmt);

	return (
		<Card className='shadow-none h-full'>
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
