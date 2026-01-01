import { LayoutDashboard, ScrollText } from 'lucide-react';
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
] as const;

type Props = Pick<Route.ComponentProps, 'matches'>;
export function AppSidebar({ matches }: Props) {
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
			<SidebarFooter />
		</Sidebar>
	);
}
