import { Check, ChevronsUpDown, LayoutList, Plus } from 'lucide-react';
import { Link, useLocation, useSubmit } from 'react-router';

import type { Project } from '~/lib/api';

import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '../ui/dropdown-menu';
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '../ui/sidebar';

type Props = {
	projects: Project[];
	currentProject: Project | undefined;
	csrfToken: string;
};
export function ProjectSwitcher({ projects, currentProject, csrfToken }: Props) {
	const { isMobile } = useSidebar();
	const submit = useSubmit();
	const location = useLocation();

	function switchTo(projectId: string) {
		if (projectId === currentProject?.id) return;

		// The action swaps the session's project and sends us back here, which
		// revalidates every loader on the page against the new project.
		submit(
			{ projectId, redirectTo: `${location.pathname}${location.search}`, _csrf: csrfToken },
			{ method: 'POST', action: '/projects/switch' },
		);
	}

	if (projects.length === 0) {
		return (
			<SidebarMenu>
				<SidebarMenuItem>
					<SidebarMenuButton asChild size='lg' tooltip='New project'>
						<Link to='/projects/new'>
							<Plus />
							<span className='font-medium'>New project</span>
						</Link>
					</SidebarMenuButton>
				</SidebarMenuItem>
			</SidebarMenu>
		);
	}

	return (
		<SidebarMenu>
			<SidebarMenuItem>
				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<SidebarMenuButton size='lg' tooltip={currentProject?.name ?? 'Select a project'}>
							<div className='grid flex-1 text-left leading-tight'>
								<span className='truncate font-medium'>{currentProject?.name ?? 'Select a project'}</span>
								{currentProject && (
									<span className='truncate text-xs text-muted-foreground'>{currentProject.slug}</span>
								)}
							</div>

							<ChevronsUpDown className='ml-auto' />
						</SidebarMenuButton>
					</DropdownMenuTrigger>

					<DropdownMenuContent
						className='w-(--radix-dropdown-menu-trigger-width) min-w-56'
						align='start'
						side={isMobile ? 'bottom' : 'right'}
					>
						<DropdownMenuLabel className='text-xs text-muted-foreground'>Projects</DropdownMenuLabel>

						{projects.map((project) => (
							<DropdownMenuItem key={project.id} onSelect={() => switchTo(project.id)}>
								<span className='truncate'>{project.name}</span>
								{project.id === currentProject?.id && <Check className='ml-auto' />}
							</DropdownMenuItem>
						))}

						<DropdownMenuSeparator />

						<DropdownMenuItem asChild>
							<Link to='/projects'>
								<LayoutList />
								<span>All projects</span>
							</Link>
						</DropdownMenuItem>

						<DropdownMenuItem asChild>
							<Link to='/projects/new'>
								<Plus />
								<span>New project</span>
							</Link>
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</SidebarMenuItem>
		</SidebarMenu>
	);
}
