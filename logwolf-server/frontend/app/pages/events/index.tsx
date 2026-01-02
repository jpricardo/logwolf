import { Plus } from 'lucide-react';
import { Link } from 'react-router';
import type { Route } from './+types';

import { DeleteEventDTOSchema, EventsApi, type DeleteEventDTO } from '~/api/events';
import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';

import { EventsTable } from './components/events-table';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader() {
	return await EventsApi.getAll();
}

export async function action({ request }: Route.ActionArgs) {
	if (request.method === 'DELETE') {
		const fd = await request.formData();
		const data: Partial<DeleteEventDTO> = Object.fromEntries(fd.entries());

		const dto = DeleteEventDTOSchema.safeParse(data);
		if (dto.error) {
			console.error('invalid data', data);
			return;
		}

		return await EventsApi.delete(dto.data);
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
