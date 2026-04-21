import { Factory, KeyRound, LayoutDashboard, ScrollText, Settings } from 'lucide-react';

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
		title: 'Projects',
		url: '/projects',
		icon: Factory,
	},

	{
		title: 'Keys',
		url: '/keys',
		icon: KeyRound,
	},

	{
		title: 'Settings',
		url: '/settings',
		icon: Settings,
	},
] as const;

type Props = Pick<Route.ComponentProps, 'matches'> & { footer?: React.ReactNode };
export function AppSidebar({ matches, footer }: Props) {
	return (
		<Sidebar>
			{/* TODO - Logo */}
			<SidebarHeader />

			<SidebarContent>
				<SidebarGroup>
					<SidebarGroupLabel>Logwolf</SidebarGroupLabel>
					<SidebarGroupContent>
						<SidebarMenu>
							{items.map((item) => (
								<SidebarMenuItem key={item.title}>
									<SidebarMenuButton asChild isActive={matches.some((m) => m?.pathname.includes(item.url))}>
										<a href={item.url}>
											<item.icon />
											<span>{item.title}</span>
										</a>
									</SidebarMenuButton>
								</SidebarMenuItem>
							))}
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>
			</SidebarContent>

			<SidebarFooter>
				<SidebarGroup>
					<SidebarGroupLabel>Project</SidebarGroupLabel>
					<SidebarGroupContent>{footer}</SidebarGroupContent>
				</SidebarGroup>
			</SidebarFooter>
		</Sidebar>
	);
}
