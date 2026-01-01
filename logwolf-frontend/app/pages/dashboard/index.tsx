import { Link } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { Api } from '~/lib/api';

import { AverageDuration } from './components/average-duration';
import { ErrorRate } from './components/error-rate';
import { LogRate } from './components/log-rate';
import { TagsBarChart } from './components/tags-bar-chart';
import { TotalErrors } from './components/total-errors';
import { TotalLogs } from './components/total-logs';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader() {
	return await Api.getLogs();
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
								<TotalLogs className='flex-1' logs={loaderData} />
								<TotalErrors className='flex-1' logs={loaderData} />
								<AverageDuration className='flex-1' logs={loaderData} />
							</div>

							<div className='flex flex-row gap-4 flex-1'>
								<LogRate className='flex-1' logs={loaderData} />
								<ErrorRate className='flex-1' logs={loaderData} />
							</div>
						</div>

						<div className='flex flex-col flex-1'>
							<TagsBarChart logs={loaderData} />
						</div>
					</div>
				</Section>
			</div>
		</Page>
	);
}
