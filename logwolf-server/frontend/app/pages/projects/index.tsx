import { ArrowRight, Check, Plus } from 'lucide-react';
import { Link, useSubmit } from 'react-router';

import { Page } from '~/components/nav/page';
import { ProjectAvatar } from '~/components/nav/project-switcher';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { useProjects } from '~/hooks/use-projects';
import { cn } from '~/lib/utils';

export function meta() {
	return [{ title: 'Projects - Logwolf' }];
}

export default function Projects() {
	const submit = useSubmit();
	const csrfToken = useCsrfToken();
	const { projects, currentProject } = useProjects();

	function open(projectId: string) {
		// Reuses the switcher action so the session write and the membership
		// re-check that guards it stay in one place.
		submit({ projectId, redirectTo: '/dashboard', _csrf: csrfToken }, { method: 'POST', action: '/projects/switch' });
	}

	return (
		<Page
			title='Projects'
			description='Each project keeps its own events, API keys, members and retention.'
			actions={
				<Button asChild>
					<Link to='/projects/new'>
						<Plus />
						New project
					</Link>
				</Button>
			}
		>
			<div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
				{projects.map((project) => {
					const isCurrent = project.id === currentProject?.id;

					return (
						<button
							key={project.id}
							type='button'
							onClick={() => open(project.id)}
							className={cn(
								'group flex flex-col gap-5 rounded-lg border bg-card p-5 text-left shadow-xs shadow-black/[0.03] transition-colors hover:border-primary/40 focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none',
								isCurrent && 'border-primary/40',
							)}
						>
							<div className='flex w-full items-start justify-between gap-3'>
								<div className='flex min-w-0 items-center gap-3'>
									<ProjectAvatar name={project.name} className='size-10 text-base' />
									<div className='grid min-w-0 leading-tight'>
										<span className='truncate font-medium'>{project.name}</span>
										<code className='truncate font-mono text-xs text-muted-foreground'>{project.slug}</code>
									</div>
								</div>

								<ArrowRight className='size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary' />
							</div>

							<div className='flex w-full items-center justify-between gap-2 text-xs text-muted-foreground'>
								<div className='flex items-center gap-1.5'>
									<Badge variant={project.role === 'owner' ? 'default' : 'secondary'}>{project.role}</Badge>
									{isCurrent && (
										<Badge variant='outline'>
											<Check />
											Current
										</Badge>
									)}
								</div>

								<span className='whitespace-nowrap'>Created {new Date(project.created_at).toLocaleDateString()}</span>
							</div>
						</button>
					);
				})}
			</div>
		</Page>
	);
}
