import { Check } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { redirect, useFetcher } from 'react-router';
import z from 'zod';
import type { Route } from './+types';

import { CreateEventDTOSchema, EventsApi } from '~/api/events';
import { Page } from '~/components/nav/page';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import { Field, FieldError, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
import { Spinner } from '~/components/ui/spinner';
import { Textarea } from '~/components/ui/textarea';

import { Preview } from './components/preview';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'New Event - Logwolf' }];
}

type CreateEventFormData = {
	name: string;
	severity: string;
	tags: string; // split, trim
	data: string;
};

export async function action({ request }: Route.ActionArgs) {
	const fd = await request.formData();
	const data: Partial<CreateEventFormData> = Object.fromEntries(fd.entries());

	const splitTags = data.tags?.split(',').map((t) => t.trim()) ?? [];
	const pr = CreateEventDTOSchema.safeParse({ ...data, tags: splitTags });
	if (pr.error) return { error: z.flattenError(pr.error) };

	return await EventsApi.create(pr.data).then(() => redirect('/events'));
}

export default function Create({}: Route.ComponentProps) {
	const fetcher = useFetcher<Route.ComponentProps['actionData']>();
	const loading = fetcher.state !== 'idle';
	const fetcherError = fetcher.data?.error;

	// Preview
	const ref = useRef<HTMLFormElement>(null);
	const [data, setData] = useState<FormData>();

	useEffect(() => {
		if (!ref.current) return;
		setData(new FormData(ref.current));
	}, [ref.current]);

	return (
		<Page title='New Event'>
			<div className='flex flex-row gap-8'>
				<Card className='shadow-none flex-1'>
					<CardHeader>
						<CardTitle className='text-muted-foreground'>Form</CardTitle>
					</CardHeader>

					<CardContent>
						<Section>
							<fetcher.Form
								method='post'
								ref={ref}
								onChange={() => setData(new FormData(ref.current!))}
								className='flex flex-col gap-8'
							>
								{!!fetcherError?.formErrors.length && (
									<Alert variant='destructive'>
										<AlertTitle>Validation error!</AlertTitle>
										<AlertDescription>{fetcherError.formErrors}</AlertDescription>
									</Alert>
								)}

								<FieldGroup className='flex flex-row gap-8'>
									<Field>
										<FieldLabel htmlFor='name'>Name</FieldLabel>
										<Input id='name' name='name' type='text' required />
										<FieldError>{fetcherError?.fieldErrors.name}</FieldError>
									</Field>

									<Field>
										<FieldLabel htmlFor='severity'>Severity</FieldLabel>
										<Input id='severity' name='severity' type='text' required />
										<FieldError>{fetcherError?.fieldErrors.severity}</FieldError>
									</Field>

									<Field>
										<FieldLabel htmlFor='tags'>Tags</FieldLabel>
										<Input id='tags' name='tags' type='text' required />
										<FieldError>{fetcherError?.fieldErrors.tags}</FieldError>
									</Field>
								</FieldGroup>

								<FieldGroup>
									<Field>
										<FieldLabel htmlFor='data'>Data</FieldLabel>
										<Textarea id='data' name='data' defaultValue='{}' required />
										<FieldError>{fetcherError?.fieldErrors.data}</FieldError>
									</Field>
								</FieldGroup>

								<FieldGroup>
									<Field orientation='horizontal' className='justify-end'>
										<Button type='submit' disabled={loading}>
											{loading ? <Spinner /> : <Check />}
											Submit
										</Button>
									</Field>
								</FieldGroup>
							</fetcher.Form>
						</Section>
					</CardContent>
				</Card>

				<Card className='shadow-none flex-1 max-w-lg'>
					<CardHeader>
						<CardTitle className='text-muted-foreground'>Preview</CardTitle>
					</CardHeader>

					<CardContent>
						<Section>
							<Preview formData={data} />
						</Section>
					</CardContent>
				</Card>
			</div>
		</Page>
	);
}
