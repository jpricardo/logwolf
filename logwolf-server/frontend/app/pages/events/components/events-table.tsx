import type { LogwolfEventData } from '@logwolf/client-js';
import { Inbox, MoreHorizontal } from 'lucide-react';
import { useRef } from 'react';
import { Link, useFetcher } from 'react-router';

import { Button } from '~/components/ui/button';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Input } from '~/components/ui/input';
import { SeverityBadge } from '~/components/ui/severity-badge';
import { Spinner } from '~/components/ui/spinner';
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { locale } from '~/lib/locale';

import { TagList } from './tag-list';

const timeFormat = new Intl.DateTimeFormat(locale, { dateStyle: 'short', timeStyle: 'medium' });

type EventRowProps = { data: LogwolfEventData; csrfToken: string };
export function EventRow({ data, csrfToken }: EventRowProps) {
	const formRef = useRef(null);
	const fetcher = useFetcher();

	const loading = fetcher.state !== 'idle';

	return (
		<TableRow key={data.id} className={loading ? 'opacity-50' : undefined}>
			<TableCell className='max-w-80'>
				<Link
					to={data.id}
					className='block truncate font-mono text-[13px] font-medium underline-offset-4 hover:text-primary hover:underline'
				>
					{data.name}
				</Link>
			</TableCell>
			<TableCell>
				<SeverityBadge variant={data.severity} />
			</TableCell>
			<TableCell>
				<TagList tags={data.tags} className='flex-nowrap' />
			</TableCell>
			<TableCell className='text-right font-mono text-xs text-muted-foreground tabular-nums'>
				{data.duration ? `${data.duration} ms` : '—'}
			</TableCell>
			<TableCell className='font-mono text-xs text-muted-foreground tabular-nums'>
				<time suppressHydrationWarning dateTime={data.created_at.toISOString()}>
					{timeFormat.format(data.created_at)}
				</time>
			</TableCell>

			<TableCell>
				<div className='flex flex-row items-center justify-end'>
					<DropdownMenu modal={false}>
						<DropdownMenuTrigger asChild>
							<Button
								variant='ghost'
								size='icon-sm'
								className='size-7'
								disabled={loading}
								aria-label={`Actions for ${data.name}`}
							>
								{loading ? <Spinner /> : <MoreHorizontal />}
							</Button>
						</DropdownMenuTrigger>

						<DropdownMenuContent className='w-44' align='end'>
							<DropdownMenuLabel>Actions</DropdownMenuLabel>
							<DropdownMenuGroup>
								<DropdownMenuItem asChild>
									<Link to={data.id}>Show details</Link>
								</DropdownMenuItem>

								<DropdownMenuItem
									variant='destructive'
									onClick={() => fetcher.submit(formRef.current, { method: 'DELETE' })}
								>
									<fetcher.Form method='DELETE' ref={formRef}>
										Delete event
										<Input type='hidden' name='id' value={data.id} />
										<Input type='hidden' name='_csrf' value={csrfToken} />
									</fetcher.Form>
								</DropdownMenuItem>
							</DropdownMenuGroup>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</TableCell>
		</TableRow>
	);
}

type Props = { events: LogwolfEventData[]; csrfToken: string };
export function EventsTable({ events, csrfToken }: Props) {
	if (events.length === 0) {
		return (
			<div className='flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed bg-card/50 px-6 py-16 text-center'>
				<span className='flex size-10 items-center justify-center rounded-md border bg-muted text-muted-foreground'>
					<Inbox className='size-5' />
				</span>
				<div className='flex flex-col gap-1'>
					<p className='font-medium'>No events yet</p>
					<p className='max-w-sm text-sm text-muted-foreground'>
						Create an API key and send your first event with the SDK, or add one by hand.
					</p>
				</div>
				<div className='mt-1 flex gap-2'>
					<Button asChild variant='outline' size='sm'>
						<Link to='/keys'>Create an API key</Link>
					</Button>
					<Button asChild size='sm'>
						<Link to='/events/new'>New event</Link>
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className='overflow-hidden rounded-lg border bg-card'>
			<Table>
				<TableCaption>
					Showing the latest {events.length} event{events.length === 1 ? '' : 's'}
				</TableCaption>

				<TableHeader>
					<TableRow>
						<TableHead>Event</TableHead>
						<TableHead className='w-28'>Severity</TableHead>
						<TableHead>Tags</TableHead>
						<TableHead className='text-right'>Duration</TableHead>
						<TableHead>Created</TableHead>
						<TableHead className='w-12'>
							<span className='sr-only'>Actions</span>
						</TableHead>
					</TableRow>
				</TableHeader>

				<TableBody>
					{events.map((l) => (
						<EventRow key={l.id} data={l} csrfToken={csrfToken} />
					))}
				</TableBody>
			</Table>
		</div>
	);
}
