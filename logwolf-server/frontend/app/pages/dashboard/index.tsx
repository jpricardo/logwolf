import { Link } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { logwolf } from '~/lib/logwolf';

import { AverageDuration } from './components/average-duration';
import { ErrorRate } from './components/error-rate';
import { EventRate } from './components/event-rate';
import { TagsBarChart } from './components/tags-bar-chart';
import { TotalErrors } from './components/total-errors';
import { TotalEvents } from './components/total-events';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader({ context }: Route.LoaderArgs) {
	const ms = 1000 * 60 * 60 * 24;
	const end = new Date().getTime();
	const start = new Date().setTime(end - ms);

	const event = context.get(eventContext);
	event?.addTag('loader');
	const res = await logwolf.getAll();
	event?.set('loaderData', ['too much data']);

	const errors = res.filter((e) => e.severity === 'critical' || e.severity === 'error');

	const recentEvents = res.filter((l) => {
		const time = l.created_at.getTime();
		return time >= start && time <= end;
	});

	const recentErrors = recentEvents.filter((l) => {
		const time = l.created_at.getTime();
		return time >= start && time <= end;
	});

	return { timespan: ms, events: res, errors, recentEvents, recentErrors };
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
								<TotalEvents className='flex-1' events={events} />
								<TotalErrors className='flex-1' totalEvents={events.length} errors={errors} />
								<AverageDuration className='flex-1' events={events} />
							</div>

							<div className='flex flex-row gap-4 flex-1'>
								<EventRate className='flex-1' timespan={timespan} events={recentEvents} />
								<ErrorRate className='flex-1' timespan={timespan} events={recentErrors} />
							</div>
						</div>

						<div className='flex flex-col flex-1'>
							<TagsBarChart events={events} />
						</div>
					</div>
				</Section>
			</div>
		</Page>
	);
}
