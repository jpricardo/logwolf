import { use } from 'react';
import { Link } from 'react-router';

import type { Event } from '~/api/events';
import { Card, CardContent } from '~/components/ui/card';

type Props = { p: Promise<Event[]> };

export function RelatedEvents({ p }: Props) {
	const events = use(p);
	return (
		<Card>
			<CardContent>
				<div className='flex flex-col gap-4'>
					{events.length === 0 && <span>No related events</span>}

					{events.map((e) => {
						return (
							<Link key={e.id} to={`/events/${e.id}`} className='flex flex-col gap-0'>
								<span>
									{e.name} - {e.severity}
								</span>
								<div className='text-xs'>{e.created_at.toLocaleString()}</div>
							</Link>
						);
					})}
				</div>
			</CardContent>
		</Card>
	);
}

export function RelatedEventsSkeleton() {
	return <span>Loading...</span>;
}
