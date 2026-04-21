import { redirect } from 'react-router';

import { eventContext } from '~/context';
import { validateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';

import type { Route } from './+types';

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const session = await getSession(request.headers.get('Cookie'));

	if (request.method === 'POST') {
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		type SwitchProjectDTO = { id: string };
		const data = Object.fromEntries(fd.entries()) as SwitchProjectDTO;
		session.set('currentProjectID', data.id);
		event?.set('currentProjectID', data.id);

		// TODO - Redirecionar pra URL de origem
		return redirect('/dashboard', {
			headers: { 'Set-Cookie': await commitSession(session) },
		});
	}
}
