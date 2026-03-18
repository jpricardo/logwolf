import { data, Outlet, redirect } from 'react-router';
import type { Route } from '../+types/root';

import { AppSidebar } from '~/components/nav/app-sidebar';
import { SidebarProvider } from '~/components/ui/sidebar';
import { Toaster } from '~/components/ui/sonner';
import { getOrCreateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';
import { ThemeProvider } from '~/store/theme-provider';

export async function loader({ request }: Route.LoaderArgs) {
	const session = await getSession(request.headers.get('Cookie'));

	const user = session.get('githubUser');
	if (!user) throw redirect('/auth');

	const csrfToken = getOrCreateCsrfToken(session);

	return data({ user, csrfToken }, { headers: { 'Set-Cookie': await commitSession(session) } });
}

export default function Layout({ matches }: Route.ComponentProps) {
	return (
		<ThemeProvider>
			<SidebarProvider>
				<AppSidebar matches={matches} />

				<main className='flex px-4 py-4 w-full'>
					<Outlet />
					<Toaster />
				</main>
			</SidebarProvider>
		</ThemeProvider>
	);
}
