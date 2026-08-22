import { KeyRound, LayoutDashboard, ScrollText, Settings } from 'lucide-react';
import { Link } from 'react-router';

import type { Project } from '~/lib/api';

import type { Route } from '../../+types/root';
import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarGroup,
	SidebarGroupContent,
	SidebarGroupLabel,
	SidebarHeader,
	SidebarMenu,
	SidebarMenuButton,
	SidebarMenuItem,
} from '../ui/sidebar';
import { ProjectSwitcher } from './project-switcher';

const items = [
	{
		title: 'Dashboard',
		url: '/dashboard',
		icon: LayoutDashboard,
	},

	{
		title: 'Events',
		url: '/events',
		icon: ScrollText,
	},

	{
		title: 'Keys',
		url: '/keys',
		icon: KeyRound,
	},
] as const;

type Props = Pick<Route.ComponentProps, 'matches'> & {
	projects: Project[];
	currentProject: Project | undefined;
	csrfToken: string;
};
export function AppSidebar({ matches, projects, currentProject, csrfToken }: Props) {
	// Settings live under the project they configure. /settings still forwards
	// there, but linking straight at the project keeps the item highlighted once
	// the page is open.
	const navItems = [
		...items,
		{
			title: 'Settings',
			url: currentProject ? `/projects/${currentProject.id}/settings` : '/settings',
			icon: Settings,
		},
	];

	return (
		<Sidebar>
			<SidebarHeader>
				<ProjectSwitcher projects={projects} currentProject={currentProject} csrfToken={csrfToken} />
			</SidebarHeader>

			<SidebarContent>
				<SidebarGroup>
					<SidebarGroupLabel>Logwolf</SidebarGroupLabel>

					<SidebarGroupContent>
						<SidebarMenu>
							{navItems.map((item) => (
								<SidebarMenuItem key={item.title}>
									<SidebarMenuButton asChild isActive={matches.some((m) => m?.pathname.includes(item.url))}>
										<Link to={item.url}>
											<item.icon />
											<span>{item.title}</span>
										</Link>
									</SidebarMenuButton>
								</SidebarMenuItem>
							))}
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>
			</SidebarContent>
			<SidebarFooter />
		</Sidebar>
	);
}
