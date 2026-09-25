import { Check, Plus } from 'lucide-react';
import { Link, useSubmit } from 'react-router';

import { Page } from '~/components/nav/page';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Section } from '~/components/ui/section';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { useProjects } from '~/hooks/use-projects';

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
		<Page title='Projects'>
			<Section
				title='Projects'
				addon={
					<Button asChild>
						<Link to='/projects/new'>
							<Plus />
							New project
						</Link>
					</Button>
				}
			>
				<div className='flex flex-col gap-2'>
					{projects.map((project) => (
						<Card key={project.id} className='shadow-none py-0'>
							<CardContent className='px-0'>
								<button
									type='button'
									onClick={() => open(project.id)}
									className='flex w-full flex-row items-center justify-between gap-4 rounded-xl px-6 py-3 text-left hover:bg-accent'
								>
									<div className='flex flex-row items-center gap-3 min-w-0'>
										<span className='truncate font-medium'>{project.name}</span>
										<code className='truncate text-sm text-muted-foreground'>{project.slug}</code>

										<Badge variant={project.role === 'owner' ? 'default' : 'secondary'}>{project.role}</Badge>

										{project.id === currentProject?.id && (
											<Badge variant='outline'>
												<Check />
												Current
											</Badge>
										)}
									</div>

									<span className='text-xs text-muted-foreground whitespace-nowrap'>
										Created {new Date(project.created_at).toLocaleDateString()}
									</span>
								</button>
							</CardContent>
						</Card>
					))}
				</div>
			</Section>
		</Page>
	);
}
