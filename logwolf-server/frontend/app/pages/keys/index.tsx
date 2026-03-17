import { useFetcher } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Section } from '~/components/ui/section';
import { Api } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';

const API_URL = process.env.API_URL ?? 'http://broker:80/';
const INTERNAL_SECRET = process.env.INTERNAL_API_SECRET ?? '';
const api = new Api(API_URL, INTERNAL_SECRET);

export async function loader({ request }: Route.LoaderArgs) {
	await requireAuth(request);
	const res = await api.getKeys();
	return { keys: res };
}

export async function action({ request }: Route.ActionArgs) {
	await requireAuth(request);
	const fd = await request.formData();
	const intent = fd.get('intent');

	if (intent === 'create') {
		const res = await api.createKey('default');
		return { data: res };
	}

	if (intent === 'revoke') {
		const id = fd.get('id')?.toString() ?? '';
		await api.deleteKey(id);
		return { revoked: true };
	}

	return null;
}

export function meta() {
	return [{ title: 'API Keys - Logwolf' }];
}

type FetcherData = Awaited<ReturnType<typeof action>>;

export default function Keys({ loaderData }: Route.ComponentProps) {
	const fetcher = useFetcher<FetcherData>();
	const actionData = fetcher.data;

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
