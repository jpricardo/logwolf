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
	const event = context.get(eventContext);
	event?.addTag('loader');
	const res = await logwolf.getAll();
	event?.set('loaderData', []);
	return res;
}

export default function Dashboard({ loaderData }: Route.ComponentProps) {
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
								<TotalEvents className='flex-1' events={loaderData} />
								<TotalErrors className='flex-1' events={loaderData} />
								<AverageDuration className='flex-1' events={loaderData} />
							</div>

							<div className='flex flex-row gap-4 flex-1'>
								<EventRate className='flex-1' events={loaderData} />
								<ErrorRate className='flex-1' events={loaderData} />
							</div>
						</div>

						<div className='flex flex-col flex-1'>
							<TagsBarChart events={loaderData} />
						</div>
					</div>
				</Section>
			</div>
		</Page>
	);
}
