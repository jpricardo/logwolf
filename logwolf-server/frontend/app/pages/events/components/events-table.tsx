import { MoreHorizontal } from 'lucide-react';
import { Link } from 'react-router';

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
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';

const maxAmt = 20;

type Props = { events: Event[] };
export function EventsTable({ events }: Props) {
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
					<TableHead>Tags</TableHead>
					<TableHead className='flex flex-row items-center justify-end'>Actions</TableHead>
				</TableRow>
			</TableHeader>

			<TableBody>
				{data.map((l) => {
					return (
						<TableRow key={l.id}>
							<TableCell>
								<Link to={l.id}>{l.id}</Link>
							</TableCell>
							<TableCell>{new Date(l.created_at).toLocaleString()}</TableCell>
							<TableCell>{l.name}</TableCell>
							<TableCell>{l.severity}</TableCell>

							<TableCell>
								<div className='flex flex-row gap-2 items-center'>
									{l.tags
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
											<Button variant='ghost'>
												<MoreHorizontal />
											</Button>
										</DropdownMenuTrigger>

										<DropdownMenuContent className='w-50' side='left'>
											<DropdownMenuLabel>Actions</DropdownMenuLabel>
											<DropdownMenuGroup>
												<DropdownMenuItem asChild>
													<Link to={l.id}>Show Details</Link>
												</DropdownMenuItem>
												<DropdownMenuItem variant='destructive' onClick={() => alert('TODO')}>
													Delete Event
												</DropdownMenuItem>
											</DropdownMenuGroup>
										</DropdownMenuContent>
									</DropdownMenu>
								</div>
							</TableCell>
						</TableRow>
					);
				})}
			</TableBody>
		</Table>
	);
}
