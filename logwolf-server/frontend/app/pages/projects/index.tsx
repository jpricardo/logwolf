import { Plus } from 'lucide-react';
import { Link, redirect } from 'react-router';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';

import type { Route } from './+types';
import { ProjectsTable } from './components/projects-table';

export function meta() {
	return [{ title: 'Projects - Logwolf' }, { name: 'description', content: 'Logwolf projects!' }];
}

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);
	event?.set('user', user);

	const api = createApi(user.login);

	const res = await api.getProjects();
	event?.set('loaderData', ['too much data']);

	return res;
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const session = await getSession(request.headers.get('Cookie'));
	const user = await requireAuth(request);
	const api = createApi(user.login);

	if (request.method === 'DELETE') {
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		type DeleteProjectDTO = { id: string };

		const data = Object.fromEntries(fd.entries()) as DeleteProjectDTO;
		const res = await api.deleteProject(data.id);
		event?.set('actionData', res);

		return res;
	}

	if (request.method === 'PUT') {
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		type SwitchProjectDTO = { id: string };
		const data = Object.fromEntries(fd.entries()) as SwitchProjectDTO;
		session.set('currentProjectID', data.id);
		event?.set('currentProjectID', data.id);

		return redirect('/dashboard', {
			headers: { 'Set-Cookie': await commitSession(session) },
		});
	}
}

export default function Projects({ loaderData }: Route.ComponentProps) {
	const csrfToken = useCsrfToken();

	return (
		<Page title='Projects'>
			<div className='flex flex-col gap-8'>
				<Section
					title='My projects'
					addon={
						<Link to='/projects/new'>
							<Button>
								<Plus />
								New project
							</Button>
						</Link>
					}
				>
					<ProjectsTable projects={loaderData} csrfToken={csrfToken} />
				</Section>
			</div>
		</Page>
	);
}
