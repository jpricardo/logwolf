import type { Severity } from '@logwolf/client-js';

import { SeverityBadge } from '~/components/ui/severity-badge';
import { cn } from '~/lib/utils';

type SampleEvent = { time: string; severity: Severity; name: string; tags: string[]; duration: string };

// A static sample of what the events page shows, so the landing page renders
// without a session or a broker behind it.
const events: SampleEvent[] = [
	{ time: '14:02:11', severity: 'info', name: 'checkout.completed', tags: ['payments'], duration: '184 ms' },
	{ time: '14:02:09', severity: 'warning', name: 'cart.stock_low', tags: ['inventory'], duration: '12 ms' },
	{ time: '14:01:57', severity: 'info', name: 'user.signed_in', tags: ['auth'], duration: '96 ms' },
	{ time: '14:01:42', severity: 'error', name: 'webhook.delivery_failed', tags: ['stripe'], duration: '10.0 s' },
	{ time: '14:01:40', severity: 'info', name: 'report.exported', tags: ['reports', 'csv'], duration: '2.3 s' },
	{ time: '14:01:18', severity: 'critical', name: 'db.pool_exhausted', tags: ['api'], duration: '31 ms' },
];

export function EventFeed({ className, ...props }: React.ComponentProps<'div'>) {
	return (
		<div className={cn('overflow-hidden rounded-lg border bg-card text-sm', className)} {...props}>
			<div className='flex items-center justify-between border-b px-4 py-2'>
				<span className='font-medium'>Events</span>
				<span className='font-mono text-xs text-muted-foreground'>project: storefront</span>
			</div>

			<ul className='divide-y'>
				{events.map((e) => (
					<li key={e.time} className='grid grid-cols-[auto_5.5rem_1fr_auto] items-center gap-3 px-4 py-2.5'>
						<span className='font-mono text-xs text-muted-foreground'>{e.time}</span>
						<span>
							<SeverityBadge variant={e.severity} />
						</span>
						<span className='flex min-w-0 items-center gap-2'>
							<span className='truncate font-mono text-[13px]'>{e.name}</span>
							{e.tags.map((t) => (
								<span key={t} className='hidden rounded-xs bg-muted px-1.5 text-xs text-muted-foreground sm:inline'>
									{t}
								</span>
							))}
						</span>
						<span className='font-mono text-xs text-muted-foreground tabular-nums'>{e.duration}</span>
					</li>
				))}
			</ul>
		</div>
	);
}
