import { Check } from 'lucide-react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import type { UserProject } from '~/lib/api';

import { type SettingsActionResult, useSuccessToast } from '../action-result';

type Props = { project: UserProject; canEdit: boolean };

export function GeneralSection({ project, canEdit }: Props) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<SettingsActionResult>();
	useSuccessToast(fetcher.data);

	return (
		<Section title='General'>
			<Card className='shadow-none max-w-md'>
				<CardContent>
					<fetcher.Form method='post'>
						<FieldGroup>
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
								/>

								<FieldDescription>
									Slug: <code>{project.slug}</code> — set when the project was created and fixed after that.
								</FieldDescription>
							</Field>

							{canEdit ? (
								<Field className='flex flex-row justify-end items-end'>
									<Button type='submit' disabled={fetcher.state !== 'idle'} className='w-fit'>
										<Check />
										Save
									</Button>
								</Field>
							) : (
								<FieldDescription>Only an owner can rename this project.</FieldDescription>
							)}
						</FieldGroup>
					</fetcher.Form>
				</CardContent>
			</Card>
		</Section>
	);
}
