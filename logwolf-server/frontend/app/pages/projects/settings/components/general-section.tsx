import { Check } from 'lucide-react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import type { UserProject } from '~/lib/api';

import { type SettingsActionResult, useSuccessToast } from '../action-result';
import { SettingsFooter, SettingsRow } from './settings-row';

type Props = { project: UserProject; canEdit: boolean };

export function GeneralSection({ project, canEdit }: Props) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<SettingsActionResult>();
	useSuccessToast(fetcher.data);

	return (
		<SettingsRow title='General' description='How this project is named across the dashboard.'>
			<Card className='gap-0 overflow-hidden py-0'>
				<fetcher.Form method='post'>
					<FieldGroup className='p-5'>
						{fetcher.data?.error && (
							<Alert variant='destructive'>
								<AlertTitle>{fetcher.data.error}</AlertTitle>
							</Alert>
						)}

						<input type='hidden' name='_csrf' value={csrfToken} />
						<input type='hidden' name='intent' value='rename' />

						<Field>
							<FieldLabel htmlFor='name'>Name</FieldLabel>

							{/* Keyed on the project so opening another project's settings
							    doesn't leave the previous name in an uncontrolled input. */}
							<Input
								key={project.id}
								id='name'
								name='name'
								type='text'
								defaultValue={project.name}
								disabled={!canEdit}
								required
								className='max-w-sm'
							/>

							<FieldDescription>
								Slug: <code className='font-mono text-foreground'>{project.slug}</code> — set when the project was
								created and fixed after that.
							</FieldDescription>
						</Field>
					</FieldGroup>

					<SettingsFooter hint={canEdit ? undefined : 'Only an owner can rename this project.'}>
						{canEdit && (
							<Button type='submit' size='sm' disabled={fetcher.state !== 'idle'}>
								<Check />
								Save
							</Button>
						)}
					</SettingsFooter>
				</fetcher.Form>
			</Card>
		</SettingsRow>
	);
}
