import { Outlet } from 'react-router';
import type { Route } from '../+types/root';

import { AppSidebar } from '~/components/nav/app-sidebar';
import { SidebarProvider } from '~/components/ui/sidebar';
import { ThemeProvider } from '~/store/theme-provider';

export default function Layout({ matches }: Route.ComponentProps) {
	return (
		<ThemeProvider>
			<SidebarProvider>
				<AppSidebar matches={matches} />

				<main className='flex px-4 py-4 w-full'>
					<Outlet />
				</main>
			</SidebarProvider>
		</ThemeProvider>
	);
}
