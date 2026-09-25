import { redirect } from 'react-router';

import { eventContext } from '~/context';
import { requireAuth } from '~/lib/auth.server';
import { getCurrentProjectID } from '~/lib/session.server';

import type { Route } from './+types';

/**
 * Settings moved under the project they configure. This path stays behind as a
 * forward so bookmarks — and anything still linking to /settings — land on the
 * settings of whichever project the session is currently pointed at.
 */
export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	await requireAuth(request);

	const projectId = await getCurrentProjectID(request);
	if (!projectId) return redirect('/projects/new');

	return redirect(`/projects/${projectId}/settings`);
}
