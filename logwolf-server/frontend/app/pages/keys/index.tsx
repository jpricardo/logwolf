import { Check, Copy, KeyRound, Plus } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useFetcher } from 'react-router';

import { Page } from '~/components/nav/page';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Section } from '~/components/ui/section';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { API_KEY_SCOPES, createApi, type ApiKeyScope } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { getCurrentProjectID } from '~/lib/session.server';
import { cn } from '~/lib/utils';

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

function CopyButton({ value }: { value: string }) {
	const [copied, setCopied] = useState(false);

	useEffect(() => {
		if (!copied) return;
		const t = setTimeout(() => setCopied(false), 2000);
		return () => clearTimeout(t);
	}, [copied]);

	return (
		<Button
			type='button'
			variant='outline'
			size='sm'
			onClick={() => navigator.clipboard.writeText(value).then(() => setCopied(true))}
		>
			{copied ? <Check /> : <Copy />}
			{copied ? 'Copied' : 'Copy'}
		</Button>
	);
}

export default function Keys({ loaderData }: Route.ComponentProps) {
	const fetcher = useFetcher<FetcherData>();
	const actionData = fetcher.data;
	const csrfToken = useCsrfToken();

	if (loaderData.noProject) {
		return (
			<Page title='API keys'>
				<p className='text-sm text-muted-foreground'>Select a project to manage its API keys.</p>
			</Page>
		);
	}

	const activeCount = loaderData.keys.filter((k) => k.active).length;

	return (
		<Page
			title='API keys'
			description='Keys let your applications send events to this project, and read or delete them.'
		>
			<div className='flex flex-col gap-6'>
				{actionData?.error && (
					<Alert variant='destructive'>
						<AlertTitle>{actionData.error.message}</AlertTitle>
					</Alert>
				)}

				{actionData?.data?.key && (
					<Alert variant='warning'>
						<KeyRound />
						<AlertTitle>Copy your API key now — it won't be shown again.</AlertTitle>
						<AlertDescription className='w-full gap-3'>
							<div className='mt-1 flex w-full flex-col gap-2 sm:flex-row sm:items-center'>
								<code className='flex-1 rounded-md border bg-background px-3 py-2 font-mono text-[13px] break-all text-foreground'>
									{actionData.data.key}
								</code>
								<CopyButton value={actionData.data.key} />
							</div>
							<p className='text-xs'>Scopes: {actionData.data.scopes.join(', ')}</p>
						</AlertDescription>
					</Alert>
				)}

				<div className='grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]'>
					<Section title='Keys' description={`${activeCount} active, ${loaderData.keys.length - activeCount} revoked`}>
						{loaderData.keys.length === 0 ? (
							<div className='flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed bg-card/50 px-6 py-12 text-center'>
								<KeyRound className='size-5 text-muted-foreground' />
								<p className='text-sm text-muted-foreground'>No API keys yet. Generate one to start sending events.</p>
							</div>
						) : (
							<div className='overflow-hidden rounded-lg border bg-card'>
								<Table>
									<TableHeader>
										<TableRow>
											<TableHead>Key</TableHead>
											<TableHead>Scopes</TableHead>
											<TableHead>Created</TableHead>
											<TableHead className='w-0'>
												<span className='sr-only'>Actions</span>
											</TableHead>
										</TableRow>
									</TableHeader>

									<TableBody>
										{loaderData.keys.map((key) => (
											<TableRow key={key.id} className={key.active ? undefined : 'text-muted-foreground'}>
												<TableCell className='whitespace-normal'>
													<div className='flex flex-col gap-1'>
														<div className='flex items-center gap-2'>
															<span
																className={cn(
																	'size-1.5 shrink-0 rounded-[1px]',
																	key.active ? 'bg-chart-5' : 'bg-muted-foreground/40',
																)}
																aria-hidden
															/>
															<code className='font-mono text-[13px]'>{key.prefix}…</code>
															{!key.active && <Badge variant='outline'>revoked</Badge>}
														</div>
														{key.legacy && key.active && (
															<p className='max-w-sm text-xs text-muted-foreground'>
																Created before keys had scopes, so it keeps full access. To narrow it, generate a key
																with only the scopes you need and revoke this one.
															</p>
														)}
													</div>
												</TableCell>
												<TableCell>
													<div className='flex flex-row gap-1'>
														{key.scopes.map((scope) => (
															<Badge
																key={scope}
																variant={scope === 'ingest' ? 'secondary' : 'default'}
																className='font-mono'
															>
																{scope}
															</Badge>
														))}
													</div>
												</TableCell>
												<TableCell className='text-muted-foreground'>
													{new Date(key.created_at).toLocaleDateString()}
												</TableCell>
												<TableCell className='text-right'>
													{key.active && (
														<fetcher.Form method='post'>
															<input type='hidden' name='_csrf' value={csrfToken} />
															<input type='hidden' name='intent' value='revoke' />
															<input type='hidden' name='id' value={key.id} />
															<Button
																type='submit'
																variant='ghost'
																size='sm'
																className='text-destructive hover:bg-destructive/10 hover:text-destructive'
															>
																Revoke
															</Button>
														</fetcher.Form>
													)}
												</TableCell>
											</TableRow>
										))}
									</TableBody>
								</Table>
							</div>
						)}
					</Section>

					<Card>
						<CardHeader>
							<CardTitle>New key</CardTitle>
							<CardDescription>
								Anyone holding a key can do everything its scopes allow, and keys used in a browser can be read out of
								the page. Give read and delete only to keys that stay on a server.
							</CardDescription>
						</CardHeader>

						<CardContent>
							<fetcher.Form method='post' className='flex flex-col gap-4'>
								<input type='hidden' name='_csrf' value={csrfToken} />
								<input type='hidden' name='intent' value='create' />

								<fieldset className='flex flex-col gap-2'>
									<legend className='mb-2 text-sm font-medium'>Scopes</legend>

									{API_KEY_SCOPES.map((scope) => (
										<label
											key={scope}
											htmlFor={`scope-${scope}`}
											className='grid cursor-pointer grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 rounded-md border px-3 py-2.5 transition-colors hover:bg-accent/50 has-checked:border-primary/40 has-checked:bg-primary/5'
										>
											<input
												id={`scope-${scope}`}
												type='checkbox'
												name='scope'
												value={scope}
												defaultChecked={scope === 'ingest'}
												className='row-span-2 mt-0.5 size-4 accent-primary'
											/>
											<span className='font-mono text-sm font-medium'>{scope}</span>
											<span className='text-xs text-muted-foreground'>{SCOPE_DESCRIPTIONS[scope]}</span>
										</label>
									))}
								</fieldset>

								<Button type='submit' disabled={fetcher.state !== 'idle'}>
									<Plus />
									Generate key
								</Button>
							</fetcher.Form>
						</CardContent>
					</Card>
				</div>
			</div>
		</Page>
	);
}
