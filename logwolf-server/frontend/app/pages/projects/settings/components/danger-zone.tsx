import { Trash2 } from 'lucide-react';
import { useState } from 'react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '~/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import type { UserProject } from '~/lib/api';

import type { SettingsActionResult } from '../action-result';

type Props = { project: UserProject };

export function DangerZone({ project }: Props) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<SettingsActionResult>();

	const [open, setOpen] = useState(false);
	const [confirmation, setConfirmation] = useState('');

	// A successful delete redirects, so there is no success state to report here
	// — only the mismatch and whatever the broker refuses.
	const confirmed = confirmation === project.name;

	function onOpenChange(next: boolean) {
		setOpen(next);
		if (!next) setConfirmation('');
	}

	return (
		<Section title='Danger zone'>
			<Card className='shadow-none max-w-md border-destructive/50'>
				<CardContent className='flex flex-col gap-3'>
					<div className='flex flex-col gap-1'>
						<span className='text-sm font-medium'>Delete this project</span>
						<span className='text-sm text-muted-foreground'>
							Its events, API keys and members go with it. This cannot be undone.
						</span>
					</div>

					{fetcher.data?.error && (
						<Alert variant='destructive'>
							<AlertTitle>{fetcher.data.error}</AlertTitle>
						</Alert>
					)}

					<Button variant='destructive' className='w-fit' onClick={() => onOpenChange(true)}>
						<Trash2 />
						Delete project
					</Button>
				</CardContent>
			</Card>

			<Dialog open={open} onOpenChange={onOpenChange}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Delete {project.name}?</DialogTitle>
						<DialogDescription>
							This deletes the project along with every event, API key and membership in it.
						</DialogDescription>
					</DialogHeader>

					<fetcher.Form method='post'>
						<FieldGroup>
							<input type='hidden' name='_csrf' value={csrfToken} />
							<input type='hidden' name='intent' value='delete' />

							<Field>
								<FieldLabel htmlFor='confirmation'>
									Type <code>{project.name}</code> to confirm
								</FieldLabel>

								<Input
									id='confirmation'
									name='confirmation'
									type='text'
									autoComplete='off'
									value={confirmation}
									onChange={(e) => setConfirmation(e.target.value)}
								/>
							</Field>

							<DialogFooter>
								<Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
									Cancel
								</Button>

								<Button type='submit' variant='destructive' disabled={!confirmed || fetcher.state !== 'idle'}>
									<Trash2 />
									Delete project
								</Button>
							</DialogFooter>
						</FieldGroup>
					</fetcher.Form>
				</DialogContent>
			</Dialog>
		</Section>
	);
}
