import { type DeleteLogwolfEventDTO } from '@logwolf/client-js';
import { Plus } from 'lucide-react';
import { Link } from 'react-router';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { validateCsrfToken } from '~/lib/csrf.server';
import { logwolf } from '~/lib/logwolf';

import type { Route } from './+types';
import { EventsTable } from './components/events-table';

export function meta() {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader({ context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const res = await logwolf.getAll({ page: 1, pageSize: 20 });
	event?.set('loaderData', ['too much data']);

	return res;
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	if (request.method === 'DELETE') {
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		const data = Object.fromEntries(fd.entries()) as DeleteLogwolfEventDTO;
		const res = await logwolf.delete(data);
		event?.set('actionData', res);

		return res;
	}
}

export default function Events({ loaderData }: Route.ComponentProps) {
	const csrfToken = useCsrfToken();

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
					<EventsTable events={loaderData} csrfToken={csrfToken} />
				</Section>
			</div>
		</Page>
	);
}
