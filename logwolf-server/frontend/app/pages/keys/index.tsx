import { useFetcher } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Section } from '~/components/ui/section';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { api } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	await requireAuth(request);
	const res = await api.getKeys();
	event?.set('loaderData', res);

	return { keys: res };
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	try {
		await requireAuth(request);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		const intent = fd.get('intent');
		event?.set('intent', intent);

		if (intent === 'create') {
			const res = await api.createKey('default');
			event?.set('actionData', { ...res, key: '-' });
			return { data: res };
		}

		if (intent === 'revoke') {
			const id = fd.get('id')?.toString() ?? '';
			await api.deleteKey(id);
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

	return (
		<Page title='API Keys'>
			<div className='flex flex-col gap-8'>
				{actionData?.data?.key && (
					<Card className='border-yellow-500 bg-yellow-50 dark:bg-yellow-950 dark:border-yellow-700 shadow-none'>
						<CardContent className='flex flex-col gap-2 pt-4'>
							<p className='text-sm font-semibold text-amber-800'>Copy your API key now — it won't be shown again.</p>
							<code className='text-sm break-all text-amber-900'>{actionData.data.key}</code>
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

				<Section
					title='API Keys'
					addon={
						<fetcher.Form method='post'>
							<input type='hidden' name='_csrf' value={csrfToken} />
							<input type='hidden' name='intent' value='create' />
							<Button type='submit' disabled={fetcher.state !== 'idle'}>
								Generate new key
							</Button>
						</fetcher.Form>
					}
				>
					<div className='flex flex-col gap-2'>
						{loaderData.keys.length === 0 && <p className='text-sm text-muted-foreground'>No API keys yet.</p>}
						{loaderData.keys.map((key) => (
							<Card key={key.id} className='shadow-none'>
								<CardContent className='flex flex-row items-center justify-between py-3'>
									<div className='flex flex-row items-center gap-4'>
										<code className='text-sm'>{key.prefix}...</code>
										<Badge variant={key.active ? 'default' : 'secondary'}>{key.active ? 'active' : 'revoked'}</Badge>
										<span className='text-xs text-muted-foreground'>
											Created {new Date(key.created_at).toLocaleDateString()}
										</span>
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
