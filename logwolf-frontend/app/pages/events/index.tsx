import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Section } from '~/components/ui/section';
import { Api } from '~/lib/api';
import { LogsTable } from './components/logs-table';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader() {
	return await Api.getLogs();
}

export default function Events({ loaderData }: Route.ComponentProps) {
	return (
		<Page title='Events'>
			<div className='flex flex-col gap-8'>
				<Section title='Last events'>
					<LogsTable logs={loaderData} />
				</Section>
			</div>
		</Page>
	);
}
