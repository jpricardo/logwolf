import { Plus } from 'lucide-react';
import { useState } from 'react';
import { redirect, useFetcher } from 'react-router';

import { Page } from '~/components/nav/page';
import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
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
		<Page title='New Project'>
			<Section title={isFirstProject ? 'Welcome to Logwolf' : 'New project'}>
				<Card className='shadow-none max-w-md'>
					<CardContent>
						<fetcher.Form method='post'>
							<FieldGroup>
								{isFirstProject && (
									<p className='text-sm text-muted-foreground'>
										Projects keep events, API keys and retention settings separate. Create one to get started.
									</p>
								)}

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
												Slug: <code>{slug}</code>
											</>
										) : (
											'The slug is generated from the name.'
										)}
									</FieldDescription>
								</Field>

								<Field className='flex flex-row justify-end items-end'>
									<Button type='submit' disabled={!slug || fetcher.state !== 'idle'} className='w-fit'>
										<Plus />
										Create project
									</Button>
								</Field>
							</FieldGroup>
						</fetcher.Form>
					</CardContent>
				</Card>
			</Section>
		</Page>
	);
}
