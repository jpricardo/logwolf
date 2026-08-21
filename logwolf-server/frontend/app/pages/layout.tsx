import { data, Outlet, redirect } from 'react-router';

import { AppSidebar } from '~/components/nav/app-sidebar';
import { SidebarProvider } from '~/components/ui/sidebar';
import { Toaster } from '~/components/ui/sonner';
import { eventContext } from '~/context';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { getOrCreateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';
import { ThemeProvider } from '~/store/theme-provider';

import type { Route } from './+types/layout';

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);

	const session = await getSession(request.headers.get('Cookie'));
	const csrfToken = getOrCreateCsrfToken(session);

	const api = createApi(user.login);
	const projects = await api.getProjects();
	const url = new URL(request.url);

	// The stored project is only usable while the user is still a member of it —
	// a deleted project, or one the user was removed from, falls back to the
	// first project they can still reach.
	const storedProjectID = session.get('currentProjectID');
	const currentProject = projects.find((p) => p.id === storedProjectID) ?? projects.at(0);

	if (currentProject?.id !== storedProjectID) {
		if (currentProject) session.set('currentProjectID', currentProject.id);
		else session.unset('currentProjectID');

		// Child loaders of this request already read the stale cookie, so reload
		// the same URL to let them run against the corrected session.
		throw redirect(`${url.pathname}${url.search}`, {
			headers: { 'Set-Cookie': await commitSession(session) },
		});
	}

	// Every page under this layout is scoped to a project, so a user who has
	// none has nothing to render. Creating one is the only way forward, and that
	// page is the one place reachable without a project in session.
	if (projects.length === 0 && url.pathname !== '/projects/new') {
		throw redirect('/projects/new');
	}

	event?.set('currentProjectID', currentProject?.id ?? null);

	return data(
		{ user, csrfToken, projects, currentProject },
		{ headers: { 'Set-Cookie': await commitSession(session) } },
	);
}

export default function Layout({ matches, loaderData }: Route.ComponentProps) {
	const { projects, currentProject, csrfToken } = loaderData;

	return (
		<ThemeProvider>
			<SidebarProvider>
				<AppSidebar matches={matches} projects={projects} currentProject={currentProject} csrfToken={csrfToken} />

				<main className='flex px-4 py-4 w-full'>
					<Outlet />
					<Toaster />
				</main>
			</SidebarProvider>
		</ThemeProvider>
	);
}
