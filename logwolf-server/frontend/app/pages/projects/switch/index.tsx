import { redirect } from 'react-router';

import { eventContext } from '~/context';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';

import type { Route } from './+types';

const FALLBACK_REDIRECT = '/dashboard';

/**
 * Keeps the caller-supplied redirect target on this origin — an absolute URL or
 * a protocol-relative path would turn the switcher into an open redirect.
 */
function safeRedirect(to: string | undefined): string {
	if (!to?.startsWith('/') || to.startsWith('//') || to.startsWith('/\\')) return FALLBACK_REDIRECT;
	return to;
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const user = await requireAuth(request);
	const fd = await request.formData();

	await validateCsrfToken(request, fd);

	const projectId = fd.get('projectId')?.toString() ?? '';
	const redirectTo = safeRedirect(fd.get('redirectTo')?.toString());
	event?.set('projectId', projectId);

	// The id comes from the browser, so membership is re-checked here. A project
	// the user can no longer reach — deleted, or one they were removed from —
	// leaves the session alone; the layout loader re-points it on the way back.
	const api = createApi(user.login);
	const projects = await api.getProjects();

	if (!projects.some((p) => p.id === projectId)) {
		event?.setSeverity('warning');
		event?.set('switchRejected', 'not a member');
		return redirect(redirectTo);
	}

	const session = await getSession(request.headers.get('Cookie'));
	session.set('currentProjectID', projectId);

	return redirect(redirectTo, { headers: { 'Set-Cookie': await commitSession(session) } });
}

// Switching is a POST-only action; a stray GET has nothing to render.
export async function loader() {
	return redirect(FALLBACK_REDIRECT);
}
