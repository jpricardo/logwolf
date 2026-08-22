import { Plus, Trash2 } from 'lucide-react';
import { useEffect, useRef } from 'react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import type { ProjectMember } from '~/lib/api';

import { type SettingsActionResult, useSuccessToast } from '../action-result';

type Props = { members: ProjectMember[]; currentUser: string; canManage: boolean };

export function MembersSection({ members, currentUser, canManage }: Props) {
	const csrfToken = useCsrfToken();

	// Adding and removing get their own fetchers so a pending removal doesn't
	// grey out the add form, and each reports its own error where it happened.
	const addFetcher = useFetcher<SettingsActionResult>();
	const removeFetcher = useFetcher<SettingsActionResult>();

	useSuccessToast(addFetcher.data);
	useSuccessToast(removeFetcher.data);

	const addFormRef = useRef<HTMLFormElement>(null);

	// The fetcher revalidates the table on its own, but the login it just added
	// would otherwise stay in the box waiting to be added a second time.
	useEffect(() => {
		if (addFetcher.data?.success) addFormRef.current?.reset();
	}, [addFetcher.data]);

	// A project must always keep one owner, so the last one has no Remove button.
	// The broker refuses the call too; this only saves the round trip.
	const ownerCount = members.filter((m) => m.role === 'owner').length;
	const removing = removeFetcher.formData?.get('login')?.toString();

	return (
		<Section title='Members'>
			<div className='flex flex-col gap-2'>
				{(addFetcher.data?.error || removeFetcher.data?.error) && (
					<Alert variant='destructive'>
						<AlertTitle>{addFetcher.data?.error ?? removeFetcher.data?.error}</AlertTitle>
					</Alert>
				)}

				<Card className='shadow-none'>
					<CardContent>
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Member</TableHead>
									<TableHead>Role</TableHead>
									<TableHead>Joined</TableHead>
									{canManage && <TableHead className='w-0' />}
								</TableRow>
							</TableHeader>

							<TableBody>
								{members.map((member) => {
									const isLastOwner = member.role === 'owner' && ownerCount === 1;

									return (
										<TableRow key={member.id}>
											<TableCell className='font-medium'>
												{member.github_login}
												{member.github_login === currentUser && (
													<span className='ml-2 text-xs text-muted-foreground'>(you)</span>
												)}
											</TableCell>

											<TableCell>
												<Badge variant={member.role === 'owner' ? 'default' : 'secondary'}>{member.role}</Badge>
											</TableCell>

											<TableCell className='text-muted-foreground'>
												{new Date(member.created_at).toLocaleDateString()}
											</TableCell>

											{canManage && (
												<TableCell>
													{isLastOwner ? (
														<span className='text-xs text-muted-foreground whitespace-nowrap'>Last owner</span>
													) : (
														<removeFetcher.Form method='post'>
															<input type='hidden' name='_csrf' value={csrfToken} />
															<input type='hidden' name='intent' value='remove-member' />
															<input type='hidden' name='login' value={member.github_login} />

															<Button
																type='submit'
																variant='ghost'
																size='icon-sm'
																aria-label={`Remove ${member.github_login}`}
																disabled={removing === member.github_login}
															>
																<Trash2 />
															</Button>
														</removeFetcher.Form>
													)}
												</TableCell>
											)}
										</TableRow>
									);
								})}
							</TableBody>
						</Table>
					</CardContent>
				</Card>

				{canManage && (
					<Card className='shadow-none max-w-xl'>
						<CardContent>
							<addFetcher.Form method='post' ref={addFormRef}>
								<FieldGroup>
									<input type='hidden' name='_csrf' value={csrfToken} />
									<input type='hidden' name='intent' value='add-member' />

									<div className='flex flex-row items-end gap-3'>
										<Field className='flex-1'>
											<FieldLabel htmlFor='login'>GitHub login</FieldLabel>
											<Input id='login' name='login' type='text' placeholder='octocat' required />
										</Field>

										<Field className='w-32'>
											<FieldLabel htmlFor='role'>Role</FieldLabel>

											<Select name='role' defaultValue='member'>
												<SelectTrigger id='role' className='w-full'>
													<SelectValue placeholder='Role' />
												</SelectTrigger>

												<SelectContent>
													<SelectItem value='member'>member</SelectItem>
													<SelectItem value='owner'>owner</SelectItem>
												</SelectContent>
											</Select>
										</Field>

										<Button type='submit' disabled={addFetcher.state !== 'idle'}>
											<Plus />
											Add
										</Button>
									</div>
								</FieldGroup>
							</addFetcher.Form>
						</CardContent>
					</Card>
				)}
			</div>
		</Section>
	);
}
