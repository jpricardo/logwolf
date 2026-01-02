import { MoreHorizontal } from 'lucide-react';
import { useRef } from 'react';
import { Link, useFetcher } from 'react-router';

import type { Event } from '~/api/events';
import { Badge } from '~/components/ui/badge';
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
import { Spinner } from '~/components/ui/spinner';
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';

type EventRowProps = { data: Event };
export function EventRow({ data }: EventRowProps) {
	const formRef = useRef(null);
	const fetcher = useFetcher();

	const loading = fetcher.state !== 'idle';

	return (
		<TableRow key={data.id}>
			<TableCell>
				<Link to={data.id}>{data.id}</Link>
			</TableCell>
			<TableCell>{new Date(data.created_at).toLocaleString()}</TableCell>
			<TableCell>{data.name}</TableCell>
			<TableCell>{data.severity}</TableCell>
			<TableCell>{data.duration ? `${data.duration}ms` : '-'}</TableCell>

			<TableCell>
				<div className='flex flex-row gap-2 items-center'>
					{data.tags
						.toSorted((a, b) => a.localeCompare(b))
						.map((t) => (
							<Badge key={t} variant={t === 'error' ? 'destructive' : 'secondary'}>
								{t}
							</Badge>
						))}
				</div>
			</TableCell>

			<TableCell>
				<div className='flex flex-row items-center justify-end'>
					<DropdownMenu modal={false}>
						<DropdownMenuTrigger asChild>
							<Button variant='ghost' disabled={loading}>
								{loading ? <Spinner /> : <MoreHorizontal />}
							</Button>
						</DropdownMenuTrigger>

						<DropdownMenuContent className='w-50' side='left'>
							<DropdownMenuLabel>Actions</DropdownMenuLabel>
							<DropdownMenuGroup>
								<DropdownMenuItem asChild>
									<Link to={data.id}>Show Details</Link>
								</DropdownMenuItem>

								<DropdownMenuItem
									variant='destructive'
									onClick={() => fetcher.submit(formRef.current, { method: 'DELETE' })}
								>
									<fetcher.Form method='DELETE' ref={formRef}>
										Delete Event
										<Input type='hidden' name='id' value={data.id} />
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

type Props = { events: Event[] };
export function EventsTable({ events }: Props) {
	const maxAmt = 20;
	const data = events.slice(0, maxAmt);

	return (
		<Table>
			<TableCaption>Last {data.length} events</TableCaption>

			<TableHeader>
				<TableRow>
					<TableHead className='w-60'>ID</TableHead>
					<TableHead className='w-50'>Created at</TableHead>
					<TableHead>Name</TableHead>
					<TableHead>Severity</TableHead>
					<TableHead>Duration</TableHead>
					<TableHead>Tags</TableHead>
					<TableHead className='flex flex-row items-center justify-end'>Actions</TableHead>
				</TableRow>
			</TableHeader>

			<TableBody>
				{data.map((l) => (
					<EventRow key={l.id} data={l} />
				))}
			</TableBody>
		</Table>
	);
}
