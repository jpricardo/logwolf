import { useRef } from 'react';
import { data, Outlet, useFetcher } from 'react-router';

import { AppSidebar } from '~/components/nav/app-sidebar';
import { ProjectSelector } from '~/components/nav/project-selector';
import { DropdownMenuItem } from '~/components/ui/dropdown-menu';
import { Input } from '~/components/ui/input';
import { SidebarProvider } from '~/components/ui/sidebar';
import { Toaster } from '~/components/ui/sonner';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi, type Project } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { getOrCreateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';
import { ThemeProvider } from '~/store/theme-provider';

import type { Route } from './+types/layout';

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);
	event?.set('user', user);

	const session = await getSession(request.headers.get('Cookie'));
	const csrfToken = getOrCreateCsrfToken(session);

	const api = createApi(user.login);
	const projects = await api.getProjects();
	const foundProjectID = session.get('currentProjectID');
	const foundProject = projects.find((p) => p.id === foundProjectID);

	if (projects.length > 0 && (!foundProjectID || !foundProject)) {
		session.set('currentProjectID', projects[0].id);
	}

	const currentProjectID = session.get('currentProjectID');
	const currentProject = projects.find((p) => p.id === currentProjectID);
	event?.set('currentProject', currentProject);

	return data(
		{ user, csrfToken, projects, currentProject },
		{ headers: { 'Set-Cookie': await commitSession(session) } },
	);
}

type ProjectMenuItemProps = { data: Project; csrfToken: string };
function ProjectMenuItem({ data, csrfToken }: ProjectMenuItemProps) {
	const formRef = useRef(null);
	const fetcher = useFetcher();

	return (
		<DropdownMenuItem onClick={() => fetcher.submit(formRef.current, { method: 'POST' })}>
			<fetcher.Form action='/projects/switch' method='POST' ref={formRef}>
				{data.name}
				<Input type='hidden' name='id' value={data.id} />
				<Input type='hidden' name='_csrf' value={csrfToken} />
			</fetcher.Form>
		</DropdownMenuItem>
	);
}

export default function Layout({ matches, loaderData }: Route.ComponentProps) {
	const csrfToken = useCsrfToken();
	const { projects, currentProject } = loaderData;

	return (
		<ThemeProvider>
			<SidebarProvider>
				<AppSidebar
					matches={matches}
					footer={<ProjectSelector projects={projects} currentProject={currentProject} csrfToken={csrfToken} />}
				/>
				<main className='flex px-4 py-4 w-full'>
					<Outlet />
					<Toaster />
				</main>
			</SidebarProvider>
		</ThemeProvider>
	);
}
