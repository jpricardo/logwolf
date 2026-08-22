import { Plus } from 'lucide-react';
import { Link } from 'react-router';

import { Page } from '~/components/nav/page';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { getCurrentProjectID } from '~/lib/session.server';

import type { Route } from './+types';
import { EventsTable } from './components/events-table';

export function meta() {
	return [{ title: 'Events - Logwolf' }, { name: 'description', content: 'Logwolf events!' }];
}

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);

	const projectId = await getCurrentProjectID(request);
	if (!projectId) return { events: [], noProject: true, error: null };

	// A broker that refuses the read must not take the whole page down, so it
	// surfaces as an alert above an empty table.
	try {
		const events = await createApi(user.login).getLogs(projectId, { page: 1, pageSize: 20 });
		event?.set('loaderData', ['too much data']);

		return { events, noProject: false, error: null };
	} catch (err) {
		event?.setSeverity('error');
		event?.set('loaderError', err);

		return { events: [], noProject: false, error: (err as Error).message };
	}
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	if (request.method === 'DELETE') {
		const user = await requireAuth(request);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		// Deleting is scoped to the project in session, so an id belonging to
		// another one is a 404 rather than someone else's event disappearing.
		const projectId = await getCurrentProjectID(request);
		if (!projectId) return { error: 'No project selected.' };

		const id = fd.get('id')?.toString() ?? '';

		try {
			await createApi(user.login).deleteLog(projectId, id);
			event?.set('actionData', { deleted: id });

			return { deleted: id };
		} catch (err) {
			event?.setSeverity('error');
			event?.set('actionError', err);

			return { error: (err as Error).message };
		}
	}
}

export default function Events({ loaderData }: Route.ComponentProps) {
	const csrfToken = useCsrfToken();
	const { events, noProject, error } = loaderData;

	if (noProject) {
		return (
			<Page title='Events'>
				<p className='text-sm text-muted-foreground'>Select a project to view its events.</p>
			</Page>
		);
	}

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
					{error && (
						<Alert variant='destructive' className='mb-4'>
							<AlertTitle>Could not load events</AlertTitle>
							<AlertDescription>{error}</AlertDescription>
						</Alert>
					)}

					<EventsTable events={events} csrfToken={csrfToken} />
				</Section>
			</div>
		</Page>
	);
}
