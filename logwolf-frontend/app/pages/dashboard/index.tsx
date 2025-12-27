import type { Route } from '../../+types/root';

import { Page } from '~/components/nav/page';
import { Api, type Log } from '~/lib/api';
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
	const logs = loaderData as unknown as Log[];

	return (
		<Page title='Dashboard'>
			<div className='flex flex-col gap-4'>
				<div className='flex flex-row gap-4'>
					<div className='flex flex-col gap-4 flex-1 justify-stretch'>
						<div className='flex flex-row gap-4 flex-1'>
							<TotalLogs className='flex-1' logs={logs} />
							<TotalErrors className='flex-1' logs={logs} />
							<AverageDuration className='flex-1' logs={logs} />
						</div>

						<div className='flex flex-row gap-4 flex-1'>
							<LogRate className='flex-1' logs={logs} />
							<ErrorRate className='flex-1' logs={logs} />
						</div>
					</div>

					<div className='flex flex-col flex-1'>
						<TagsBarChart logs={logs} />
					</div>
				</div>
			</div>
		</Page>
	);
}
