import { useFetcher } from 'react-router';

import { Page } from '~/components/nav/page';
import { Alert, AlertTitle } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import {
	Field,
	FieldContent,
	FieldDescription,
	FieldGroup,
	FieldLabel,
	FieldLegend,
	FieldSet,
} from '~/components/ui/field';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { API_KEY_SCOPES, createApi, type ApiKeyScope } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { getCurrentProjectID } from '~/lib/session.server';

import type { Route } from './+types';

const SCOPE_DESCRIPTIONS: Record<ApiKeyScope, string> = {
	ingest: 'Send events (POST /logs, POST /logs/batch).',
	read: 'Read every event in the project (GET /logs).',
	delete: 'Delete events by filter, up to all of them at once (DELETE /logs).',
};

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);

	// Keys are listed for the project the session is pointed at; switching
	// projects revalidates this loader into that project's keys instead.
	const projectId = await getCurrentProjectID(request);
	if (!projectId) return { keys: [], noProject: true };

	const api = createApi(user.login);
	const res = await api.getKeys(projectId);
	event?.set('loaderData', res);

	return { keys: res, noProject: false };
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	try {
		const user = await requireAuth(request);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		const intent = fd.get('intent');
		event?.set('intent', intent);

		const api = createApi(user.login);

		// Both intents act on the project in session rather than one named by the
		// form, so a tab left open on a since-switched project cannot mint or
		// revoke a key somewhere the user is no longer looking.
		const projectId = await getCurrentProjectID(request);
		if (!projectId) return { error: new Error('No project selected.') };

		if (intent === 'create') {
			// Unknown values are left for the broker to refuse.
			const scopes = fd.getAll('scope').map(String) as ApiKeyScope[];
			if (scopes.length === 0) return { error: new Error('Pick at least one scope.') };

			const res = await api.createKey(projectId, scopes);
			event?.set('actionData', { ...res, key: '-' });
			return { data: res };
		}

		if (intent === 'revoke') {
			const id = fd.get('id')?.toString() ?? '';
			await api.deleteKey(projectId, id);
			event?.set('actionData', null);
			return { revoked: true };
		}

		return null;
	} catch (err) {
		event?.setSeverity('error');
		event?.set('actionError', err);
		return { error: err as Error };
	}
}

export function meta() {
	return [{ title: 'API Keys - Logwolf' }];
}

type FetcherData = Awaited<ReturnType<typeof action>>;

export default function Keys({ loaderData }: Route.ComponentProps) {
	const fetcher = useFetcher<FetcherData>();
	const actionData = fetcher.data;
	const csrfToken = useCsrfToken();

	if (loaderData.noProject) {
		return (
			<Page title='API Keys'>
				<p className='text-sm text-muted-foreground'>Select a project to manage its API keys.</p>
			</Page>
		);
	}

	return (
		<Page title='API Keys'>
			<div className='flex flex-col gap-8'>
				{actionData?.error && (
					<Alert variant='destructive'>
						<AlertTitle>{actionData.error.message}</AlertTitle>
					</Alert>
				)}

				{actionData?.data?.key && (
					<Card className='border-yellow-500 bg-yellow-50 dark:bg-yellow-950 dark:border-yellow-700 shadow-none'>
						<CardContent className='flex flex-col gap-2 pt-4'>
							<p className='text-sm font-semibold text-amber-800'>Copy your API key now — it won't be shown again.</p>
							<code className='text-sm break-all text-amber-900'>{actionData.data.key}</code>
							<p className='text-xs text-amber-800'>Scopes: {actionData.data.scopes.join(', ')}</p>
							<Button
								variant='outline'
								className='self-start'
								onClick={() => navigator.clipboard.writeText(actionData.data.key)}
							>
								Copy to clipboard
							</Button>
						</CardContent>
					</Card>
				)}

				<Section title='New key'>
					<Card className='shadow-none max-w-xl'>
						<CardContent>
							<fetcher.Form method='post'>
								<input type='hidden' name='_csrf' value={csrfToken} />
								<input type='hidden' name='intent' value='create' />
								<FieldGroup>
									<FieldSet>
										<FieldLegend variant='label'>Scopes</FieldLegend>
										<FieldDescription>
											Anyone holding a key can do everything its scopes allow, and keys used in a browser can be read
											out of the page. Give read and delete only to keys that stay on a server.
										</FieldDescription>
										<FieldGroup data-slot='checkbox-group'>
											{API_KEY_SCOPES.map((scope) => (
												<Field key={scope} orientation='horizontal'>
													<input
														id={`scope-${scope}`}
														type='checkbox'
														name='scope'
														value={scope}
														defaultChecked={scope === 'ingest'}
														className='mt-0.5 size-4 accent-primary'
													/>
													<FieldContent>
														<FieldLabel htmlFor={`scope-${scope}`}>{scope}</FieldLabel>
														<FieldDescription>{SCOPE_DESCRIPTIONS[scope]}</FieldDescription>
													</FieldContent>
												</Field>
											))}
										</FieldGroup>
									</FieldSet>
									<Field className='flex flex-row justify-end items-end'>
										<Button type='submit' disabled={fetcher.state !== 'idle'} className='w-fit'>
											Generate new key
										</Button>
									</Field>
								</FieldGroup>
							</fetcher.Form>
						</CardContent>
					</Card>
				</Section>

				<Section title='API Keys'>
					<div className='flex flex-col gap-2'>
						{loaderData.keys.length === 0 && <p className='text-sm text-muted-foreground'>No API keys yet.</p>}
						{loaderData.keys.map((key) => (
							<Card key={key.id} className='shadow-none'>
								<CardContent className='flex flex-row items-center justify-between py-3'>
									<div className='flex flex-col gap-2'>
										<div className='flex flex-row flex-wrap items-center gap-4'>
											<code className='text-sm'>{key.prefix}...</code>
											<Badge variant={key.active ? 'default' : 'secondary'}>{key.active ? 'active' : 'revoked'}</Badge>
											<div className='flex flex-row gap-1'>
												{key.scopes.map((scope) => (
													<Badge key={scope} variant='outline'>
														{scope}
													</Badge>
												))}
											</div>
											<span className='text-xs text-muted-foreground'>
												Created {new Date(key.created_at).toLocaleDateString()}
											</span>
										</div>
										{key.legacy && key.active && (
											<p className='text-xs text-muted-foreground'>
												Created before keys had scopes, so it keeps full access. To narrow it, generate a key with only
												the scopes you need and revoke this one.
											</p>
										)}
									</div>
									{key.active && (
										<fetcher.Form method='post'>
											<input type='hidden' name='_csrf' value={csrfToken} />
											<input type='hidden' name='intent' value='revoke' />
											<input type='hidden' name='id' value={key.id} />
											<Button type='submit' variant='destructive' size='sm'>
												Revoke
											</Button>
										</fetcher.Form>
									)}
								</CardContent>
							</Card>
						))}
					</div>
				</Section>
			</div>
		</Page>
	);
}
