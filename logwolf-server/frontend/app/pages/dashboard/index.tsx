import { Suspense } from 'react';
import { Link } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { logwolf } from '~/lib/logwolf';

import { AverageDuration, AverageDurationSkeleton } from './components/average-duration';
import { ErrorRate, ErrorRateSkeleton } from './components/error-rate';
import { EventRate, EventRateSkeleton } from './components/event-rate';
import { TagsBarChart, TagsBarChartSkeleton } from './components/tags-bar-chart';
import { TotalErrors, TotalErrorsSkeleton } from './components/total-errors';
import { TotalEvents, TotalEventsSkeleton } from './components/total-events';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader({ context }: Route.LoaderArgs) {
	const ms = 1000 * 60 * 60 * 24;
	const end = new Date().getTime();
	const start = new Date().setTime(end - ms);

	const event = context.get(eventContext);
	event?.addTag('loader');
	const res = logwolf.getAll();

	const errors = res.then((r) => r.filter((e) => e.severity === 'critical' || e.severity === 'error'));

	const recentEvents = res.then((r) =>
		r.filter((l) => {
			const time = l.created_at.getTime();
			return time >= start && time <= end;
		}),
	);

	const recentErrors = recentEvents.then((r) =>
		r.filter((l) => {
			const time = l.created_at.getTime();
			return time >= start && time <= end;
		}),
	);

	const data = { timespan: ms, events: res, errors, recentEvents, recentErrors };
	event?.set('loaderData', ['too much data']);
	return data;
}

export default function Dashboard({ loaderData }: Route.ComponentProps) {
	const { events, errors, recentEvents, recentErrors, timespan } = loaderData;
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
					<div className='flex flex-row gap-4'>
						<div className='flex flex-col gap-4 flex-1 justify-stretch'>
							<div className='flex flex-row gap-4 flex-1'>
								<Suspense fallback={<TotalEventsSkeleton className='flex-1' />}>
									<TotalEvents className='flex-1' p={events} />
								</Suspense>

								<Suspense fallback={<TotalErrorsSkeleton className='flex-1' />}>
									<TotalErrors className='flex-1' p={errors} />
								</Suspense>

								<Suspense fallback={<AverageDurationSkeleton className='flex-1' />}>
									<AverageDuration className='flex-1' p={events} />
								</Suspense>
							</div>

							<div className='flex flex-row gap-4 flex-1'>
								<Suspense fallback={<EventRateSkeleton className='flex-1' />}>
									<EventRate className='flex-1' timespan={timespan} p={recentEvents} />
								</Suspense>

								<Suspense fallback={<ErrorRateSkeleton className='flex-1' />}>
									<ErrorRate className='flex-1' timespan={timespan} p={recentErrors} />
								</Suspense>
							</div>
						</div>

						<div className='flex flex-col flex-1'>
							<Suspense fallback={<TagsBarChartSkeleton />}>
								<TagsBarChart p={events} />
							</Suspense>
						</div>
					</div>
				</Section>
			</div>
		</Page>
	);
}
