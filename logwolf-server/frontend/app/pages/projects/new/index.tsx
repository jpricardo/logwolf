import { Plus } from 'lucide-react';
import { useState } from 'react';
import { redirect, useFetcher } from 'react-router';

import { Page } from '~/components/nav/page';
import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { useProjects } from '~/hooks/use-projects';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { commitSession, getSession } from '~/lib/session.server';
import { slugify } from '~/lib/slug';

import type { Route } from './+types';

export function meta() {
	return [{ title: 'New Project - Logwolf' }];
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const user = await requireAuth(request);
	const fd = await request.formData();

	await validateCsrfToken(request, fd);

	const name = fd.get('name')?.toString().trim() ?? '';
	// The slug is derived here, not read off the form: what the user saw while
	// typing is only a preview, and the browser doesn't get to pick the slug.
	const slug = slugify(name);
	event?.set('slug', slug);

	if (!name) return { error: 'Name is required.' };
	if (!slug) return { error: 'Name must contain at least one letter or number.' };

	try {
		const api = createApi(user.login);
		const project = await api.createProject(name, slug);
		event?.set('actionData', project);

		// A project you just created is the one you want to be looking at.
		const session = await getSession(request.headers.get('Cookie'));
		session.set('currentProjectID', project.id);

		return redirect('/dashboard', { headers: { 'Set-Cookie': await commitSession(session) } });
	} catch (err) {
		event?.setSeverity('error');
		event?.set('actionError', err);
		return { error: (err as Error).message };
	}
}

export default function NewProject() {
	const fetcher = useFetcher<Route.ComponentProps['actionData']>();
	const csrfToken = useCsrfToken();
	const { projects } = useProjects();

	const [name, setName] = useState('');
	const slug = slugify(name);

	// Anyone without a project was sent here by the layout loader rather than
	// arriving on purpose, so the page explains itself the first time around.
	const isFirstProject = projects.length === 0;

	return (
		<Page
			title={isFirstProject ? 'Welcome to Logwolf' : 'New project'}
			parents={isFirstProject ? [] : [{ label: 'Projects', to: '/projects' }]}
			description={
				isFirstProject
					? 'Projects keep events, API keys and retention settings separate. Create one to get started.'
					: 'A fresh space for another application, with its own keys, members and retention.'
			}
		>
			<Card className='max-w-lg gap-0 overflow-hidden py-0'>
				<fetcher.Form method='post'>
					<FieldGroup className='p-5'>
						{fetcher.data?.error && (
							<Alert variant='destructive'>
								<AlertTitle>{fetcher.data.error}</AlertTitle>
							</Alert>
						)}

						<input type='hidden' name='_csrf' value={csrfToken} />

						<Field>
							<FieldLabel htmlFor='name'>Name</FieldLabel>

							<Input
								id='name'
								name='name'
								type='text'
								placeholder='My Application'
								value={name}
								onChange={(e) => setName(e.target.value)}
								required
							/>

							<FieldDescription>
								{slug ? (
									<>
										Slug: <code className='font-mono text-foreground'>{slug}</code>
									</>
								) : (
									'The slug is generated from the name.'
								)}
							</FieldDescription>
						</Field>
					</FieldGroup>

					<div className='flex justify-end border-t bg-muted/40 px-5 py-3'>
						<Button type='submit' size='sm' disabled={!slug || fetcher.state !== 'idle'}>
							<Plus />
							Create project
						</Button>
					</div>
				</fetcher.Form>
			</Card>
		</Page>
	);
}
