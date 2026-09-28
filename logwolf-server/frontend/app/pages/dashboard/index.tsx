import { ArrowRight } from 'lucide-react';
import { Suspense } from 'react';
import { Link } from 'react-router';

import { Page } from '~/components/nav/page';
import { Button } from '~/components/ui/button';
import { eventContext } from '~/context';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { getCurrentProjectID } from '~/lib/session.server';

import type { Route } from './+types';
import { AverageDuration, AverageDurationSkeleton } from './components/average-duration';
import { ErrorRate, ErrorRateSkeleton } from './components/error-rate';
import { EventRate, EventRateSkeleton } from './components/event-rate';
import { TagsBarChart, TagsBarChartSkeleton } from './components/tags-bar-chart';
import { TotalErrors, TotalErrorsSkeleton } from './components/total-errors';
import { TotalEvents, TotalEventsSkeleton } from './components/total-events';

export function meta() {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);

	const projectId = await getCurrentProjectID(request);
	if (!projectId) return { metrics: null };

	const api = createApi(user.login);
	const metrics = api.getMetrics(projectId);
	event?.set('loaderData', 'async data');

	return { metrics };
}

export default function Dashboard({ loaderData }: Route.ComponentProps) {
	const { metrics } = loaderData;

	if (!metrics) {
		return (
			<Page title='Dashboard'>
				<p className='text-sm text-muted-foreground'>Select a project to view its dashboard.</p>
			</Page>
		);
	}

	return (
		<Page
			title='Dashboard'
			description='How much this project logs, and how much of it is going wrong.'
			actions={
				<Button asChild variant='outline'>
					<Link to='/events'>
						View events
						<ArrowRight />
					</Link>
				</Button>
			}
		>
			{/* On wide screens the rate tiles stack in the first column and the
			    chart fills the two beside them. */}
			<div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
				<Suspense fallback={<TotalEventsSkeleton />}>
					<TotalEvents p={metrics} />
				</Suspense>

				<Suspense fallback={<TotalErrorsSkeleton />}>
					<TotalErrors p={metrics} />
				</Suspense>

				<Suspense fallback={<AverageDurationSkeleton />}>
					<AverageDuration p={metrics} />
				</Suspense>

				<Suspense fallback={<EventRateSkeleton />}>
					<EventRate p={metrics} />
				</Suspense>

				<Suspense fallback={<ErrorRateSkeleton />}>
					<ErrorRate p={metrics} />
				</Suspense>

				<Suspense fallback={<TagsBarChartSkeleton className={chartPlacement} />}>
					<TagsBarChart className={chartPlacement} p={metrics} />
				</Suspense>
			</div>
		</Page>
	);
}

const chartPlacement = 'md:col-span-2 xl:col-span-2 xl:col-start-2 xl:row-span-2 xl:row-start-2';
