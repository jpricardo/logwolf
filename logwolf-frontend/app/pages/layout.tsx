import { Outlet } from 'react-router';
import type { Route } from '../+types/root';

import { AppSidebar } from '~/components/nav/app-sidebar';
import { SidebarProvider } from '~/components/ui/sidebar';

export default function Layout({ matches }: Route.ComponentProps) {
	return (
		<SidebarProvider>
			<AppSidebar matches={matches} />

			<main className='flex px-4 py-4'>
				<Outlet />
			</main>
		</SidebarProvider>
	);
}
