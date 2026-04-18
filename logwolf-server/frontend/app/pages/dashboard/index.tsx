import { Suspense } from 'react';
import { Link } from 'react-router';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { api } from '~/lib/api';

import type { Route } from './+types';
import { AverageDuration, AverageDurationSkeleton } from './components/average-duration';
import { ErrorRate, ErrorRateSkeleton } from './components/error-rate';
import { EventRate, EventRateSkeleton } from './components/event-rate';
import { TagsBarChart, TagsBarChartSkeleton } from './components/tags-bar-chart';
import { TotalErrors, TotalErrorsSkeleton } from './components/total-errors';
import { TotalEvents, TotalEventsSkeleton } from './components/total-events';

export function meta() {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader({ context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const metrics = api.getMetrics();
	event?.set('loaderData', 'async data');

	return { metrics };
}

export default function Dashboard({ loaderData }: Route.ComponentProps) {
	const { metrics } = loaderData;

	return (
		<Page title='Dashboard'>
			<div className='flex flex-col gap-8'>
				<Section
					title='Metrics'
					id='metrics'
					addon={
						<Button asChild>
							<Link to='/events'>Show events</Link>
						</Button>
					}
				>
					<div className='flex flex-row flex-wrap gap-4'>
						<div className='flex flex-col flex-wrap gap-4 flex-3 justify-stretch'>
							<div className='flex flex-row flex-wrap gap-4 flex-1'>
								<Suspense fallback={<TotalEventsSkeleton className='flex-1 min-w-xs' />}>
									<TotalEvents className='flex-1 min-w-xs' p={metrics} />
								</Suspense>

								<Suspense fallback={<TotalErrorsSkeleton className='flex-1 min-w-xs' />}>
									<TotalErrors className='flex-1 min-w-xs' p={metrics} />
								</Suspense>

								<Suspense fallback={<AverageDurationSkeleton className='flex-1 min-w-xs' />}>
									<AverageDuration className='flex-1 min-w-xs' p={metrics} />
								</Suspense>
							</div>

							<div className='flex flex-row flex-wrap gap-4 flex-1'>
								<Suspense fallback={<EventRateSkeleton className='flex-1 min-w-xs' />}>
									<EventRate className='flex-1 min-w-xs' p={metrics} />
								</Suspense>

								<Suspense fallback={<ErrorRateSkeleton className='flex-1 min-w-xs' />}>
									<ErrorRate className='flex-1 min-w-xs' p={metrics} />
								</Suspense>
							</div>
						</div>

						<div className='flex flex-col flex-wrap flex-2'>
							<Suspense fallback={<TagsBarChartSkeleton className='flex-1 min-w-xs min-h-100' />}>
								<TagsBarChart className='flex-1 min-w-xs min-h-100' p={metrics} />
							</Suspense>
						</div>
					</div>
				</Section>
			</div>
		</Page>
	);
}
