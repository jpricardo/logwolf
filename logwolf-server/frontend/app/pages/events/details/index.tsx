import { Suspense } from 'react';
import { redirect } from 'react-router';
import type { Route } from './+types';

import { Page } from '~/components/nav/page';
import { Badge } from '~/components/ui/badge';
import { Card, CardContent } from '~/components/ui/card';
import { JSONBlock } from '~/components/ui/json-block';
import { Section } from '~/components/ui/section';
import { SeverityBadge } from '~/components/ui/severity-badge';
import { logwolf } from '~/lib/logwolf';

import { InfoItem } from './components/info-item';
import { RelatedEvents, RelatedEventsSkeleton } from './components/related-events';

export function meta({ loaderData }: Route.MetaArgs) {
	return [{ title: loaderData.name + ' - Logwolf' }, { name: 'description', content: 'Logwolf event details!' }];
}

export async function loader({ params }: Route.LoaderArgs) {
	const log = await logwolf.getOne(params.id);
	if (!log) throw redirect('/events');

	return {
		...log,
		related: logwolf.getRelated(log.id, 10),
	};
}

export default function Details({ params, loaderData }: Route.ComponentProps) {
	return (
		<Page title={`Events - ${params.id}`}>
			<div className='flex flex-row gap-8'>
				<div className='flex flex-col flex-4/5 gap-8'>
					<Section title='Event details'>
						<div className='flex flex-row gap-4'>
							<Card className='flex-1 shadow-none'>
								<CardContent className='flex flex-col gap-0'>
									<InfoItem label='ID' value={loaderData.id} className='border-b pb-2' />
									<InfoItem label='Name' value={loaderData.name} className='pt-2' />
								</CardContent>
							</Card>

							<Card className='flex-1 shadow-none'>
								<CardContent className='flex flex-col gap-0'>
									<InfoItem
										label='Created at'
										value={loaderData.created_at.toLocaleString()}
										className='border-b pb-2'
									/>
									<InfoItem
										label='Duration'
										value={loaderData.duration !== undefined ? `${loaderData.duration}ms` : '-'}
										className='pt-2'
									/>
								</CardContent>
							</Card>

							<Card className='flex-1 shadow-none'>
								<CardContent className='flex flex-col gap-0'>
									<InfoItem
										label='Severity'
										value={<SeverityBadge variant={loaderData.severity} />}
										className='border-b pb-2'
									/>
									<InfoItem
										label='Tags'
										value={
											<div className='flex flex-row gap-2 items-center'>
												{loaderData.tags.map((t) => (
													<Badge key={t} variant={t === 'error' ? 'destructive' : 'secondary'}>
														{t}
													</Badge>
												))}
											</div>
										}
										className='pt-2'
									/>
								</CardContent>
							</Card>
						</div>
					</Section>
					<Section title='Event data'>
						<JSONBlock data={loaderData.data} />
					</Section>
				</div>

				<div className='flex flex-col flex-1/5 gap-8'>
					<Section title='Related events'>
						<Suspense fallback={<RelatedEventsSkeleton />}>
							<RelatedEvents p={loaderData.related} />
						</Suspense>
					</Section>
				</div>
			</div>
		</Page>
	);
}
