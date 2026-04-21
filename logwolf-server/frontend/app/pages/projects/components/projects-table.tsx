import { MoreHorizontal } from 'lucide-react';
import { useRef } from 'react';
import { useFetcher } from 'react-router';

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
import type { Project } from '~/lib/api';

type ProjectRowProps = { data: Project; csrfToken: string };
export function ProjectRow({ data, csrfToken }: ProjectRowProps) {
	const formRef = useRef(null);
	const fetcher = useFetcher();

	const loading = fetcher.state !== 'idle';

	return (
		<TableRow key={data.id}>
			<TableCell>{data.name}</TableCell>
			<TableCell>{data.created_at.toLocaleString()}</TableCell>
			<TableCell>{data.slug}</TableCell>

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
								<DropdownMenuItem onClick={() => fetcher.submit(formRef.current, { method: 'POST' })}>
									<fetcher.Form action='/projects/switch' method='POST' ref={formRef}>
										Switch to Project
										<Input type='hidden' name='id' value={data.id} />
										<Input type='hidden' name='_csrf' value={csrfToken} />
									</fetcher.Form>
								</DropdownMenuItem>

								<DropdownMenuItem
									variant='destructive'
									onClick={() =>
										window.confirm('Are you sure?') && fetcher.submit(formRef.current, { method: 'DELETE' })
									}
								>
									<fetcher.Form method='DELETE' ref={formRef}>
										Delete Project
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

type Props = { projects: Project[]; csrfToken: string };
export function ProjectsTable({ projects, csrfToken }: Props) {
	return (
		<Table>
			<TableCaption>{projects.length} projects</TableCaption>

			<TableHeader>
				<TableRow>
					<TableHead className='w-80'>Name</TableHead>
					<TableHead>Created at</TableHead>
					<TableHead>Slug</TableHead>
					<TableHead className='flex flex-row items-center justify-end'>Actions</TableHead>
				</TableRow>
			</TableHeader>

			<TableBody>
				{projects.map((p) => (
					<ProjectRow key={p.id} data={p} csrfToken={csrfToken} />
				))}
			</TableBody>
		</Table>
	);
}
