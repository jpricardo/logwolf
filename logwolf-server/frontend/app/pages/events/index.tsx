import { type DeleteLogwolfEventDTO } from '@jpricardo/logwolf-client-js';
import { Plus } from 'lucide-react';
import { Link } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { logwolf } from '~/lib/logwolf';

import { EventsTable } from './components/events-table';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader() {
	return await logwolf.getAll();
}

export async function action({ request }: Route.ActionArgs) {
	if (request.method === 'DELETE') {
		const fd = await request.formData();
		const data = Object.fromEntries(fd.entries()) as DeleteLogwolfEventDTO;

		return await logwolf.delete(data);
	}
}

export default function Events({ loaderData }: Route.ComponentProps) {
	return (
		<Page title='Events'>
			<div className='flex flex-col gap-8'>
				<Section
					title='Last events'
					addon={
						<Link to='/events/new'>
							<Button>
								<Plus />
								New event
							</Button>
						</Link>
					}
				>
					<EventsTable events={loaderData} />
				</Section>
			</div>
		</Page>
	);
}
