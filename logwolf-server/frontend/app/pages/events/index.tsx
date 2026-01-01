import type { Route } from './+types';

import { EventsApi } from '~/api/events';
import { Page } from '~/components/nav/page';
import { Section } from '~/components/ui/section';
import { EventsTable } from './components/events-table';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader() {
	return await EventsApi.getAll();
}

export default function Events({ loaderData }: Route.ComponentProps) {
	return (
		<Page title='Events'>
			<div className='flex flex-col gap-8'>
				<Section title='Last events'>
					<EventsTable events={loaderData} />
				</Section>
			</div>
		</Page>
	);
}
