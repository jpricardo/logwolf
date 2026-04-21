import { ChevronDown, Plus } from 'lucide-react';
import { useRef } from 'react';
import { Link, useFetcher } from 'react-router';

import type { Project } from '~/lib/api';

import { Button } from '../ui/button';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '../ui/dropdown-menu';
import { Input } from '../ui/input';

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

type Props = {
	projects: Project[];
	currentProject: Project | undefined;
	csrfToken: string;
};
export function ProjectSelector({ projects, currentProject, csrfToken }: Props) {
	return (
		<DropdownMenu>
			{/* TODO - Fix Hydration error */}
			{projects.length > 0 ? (
				<DropdownMenuTrigger asChild>
					<Button variant={currentProject ? 'outline' : 'default'} className='w-full'>
						{currentProject?.name ?? 'Select a project'} <ChevronDown />
					</Button>
				</DropdownMenuTrigger>
			) : (
				<Link to='/projects/new'>
					<Button className='w-full'>
						<Plus /> New Project
					</Button>
				</Link>
			)}

			<DropdownMenuContent align='start' className='min-w-xs'>
				<DropdownMenuGroup>
					<DropdownMenuLabel>Projects</DropdownMenuLabel>
					{projects.map((p) => {
						return <ProjectMenuItem key={p.id} data={p} csrfToken={csrfToken} />;
					})}
				</DropdownMenuGroup>

				{projects.length > 0 && <DropdownMenuSeparator />}

				<DropdownMenuGroup>
					<Link to='/projects/new'>
						<DropdownMenuItem>
							<Plus /> New project
						</DropdownMenuItem>
					</Link>
				</DropdownMenuGroup>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
