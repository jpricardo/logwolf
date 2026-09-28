import { redirect } from 'react-router';

import { Page } from '~/components/nav/page';
import { Card } from '~/components/ui/card';
import { JSONBlock } from '~/components/ui/json-block';
import { Section } from '~/components/ui/section';
import { SeverityBadge } from '~/components/ui/severity-badge';
import { eventContext } from '~/context';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { locale } from '~/lib/locale';
import { getCurrentProjectID } from '~/lib/session.server';

import { TagList } from '../components/tag-list';
import type { Route } from './+types';
import { InfoItem } from './components/info-item';

export function meta({ loaderData }: Route.MetaArgs) {
	return [{ title: loaderData.name + ' - Logwolf' }, { name: 'description', content: 'Logwolf event details!' }];
}

export async function loader({ request, params, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);

	const projectId = await getCurrentProjectID(request);
	if (!projectId) throw redirect('/projects/new');

	// An event of another project is a 404 here, and a 404 reads the same as any
	// other failure: there is nothing to show, so fall back to the list.
	const log = await createApi(user.login)
		.getLog(projectId, params.id)
		.catch((err: unknown) => {
			event?.setSeverity('error');
			event?.set('loaderError', err);

			return null;
		});
	if (!log) throw redirect('/events');

	event?.set('loaderData', ['too much data']);

	return log;
}

export default function Details({ loaderData }: Route.ComponentProps) {
	const event = loaderData;

	return (
		<Page
			title={event.name}
			parents={[{ label: 'Events', to: '/events' }]}
			heading={
				<div className='flex min-w-0 flex-wrap items-center gap-3'>
					<h1 className='truncate font-mono text-2xl font-semibold tracking-tight'>{event.name}</h1>
					<SeverityBadge variant={event.severity} />
				</div>
			}
			description={
				<time suppressHydrationWarning dateTime={event.created_at.toISOString()}>
					{event.created_at.toLocaleString(locale, { dateStyle: 'full', timeStyle: 'medium' })}
				</time>
			}
		>
			<div className='grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]'>
				<Card className='h-fit gap-0 py-0'>
					<dl className='divide-y'>
						<InfoItem label='ID' value={<span className='font-mono text-xs break-all'>{event.id}</span>} />
						<InfoItem
							label='Duration'
							value={
								<span className='font-mono text-xs tabular-nums'>
									{event.duration !== undefined ? `${event.duration} ms` : '—'}
								</span>
							}
						/>
						<InfoItem
							label='Created'
							value={<span className='font-mono text-xs'>{event.created_at.toISOString()}</span>}
						/>
						<InfoItem label='Tags' value={<TagList tags={event.tags} />} />
					</dl>
				</Card>

				<Section title='Data' description='The payload attached to this event.'>
					<JSONBlock data={event.data} className='max-h-150' />
				</Section>
			</div>
		</Page>
	);
}
