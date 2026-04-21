import { Check } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { redirect, useFetcher } from 'react-router';
import z, { ZodError } from 'zod';

import { Page } from '~/components/nav/page';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import { Field, FieldError, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Section } from '~/components/ui/section';
import { Spinner } from '~/components/ui/spinner';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';

import type { Route } from './+types';

export function meta() {
	return [{ title: 'New Event - Logwolf' }];
}

const FormDataSchema = z.object({
	name: z.string(),
	slug: z.string(),
});

type CreateEventFormData = z.input<typeof FormDataSchema>;

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	const user = await requireAuth(request);

	try {
		const api = createApi(user.login);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		const d = FormDataSchema.decode(Object.fromEntries(fd.entries()) as CreateEventFormData);
		const res = await api.createProject(d).then(() => redirect('/projects'));
		event?.set('actionData', res);

		return res;
	} catch (err) {
		event?.setSeverity('error');
		event?.set('actionError', err);

		if (err instanceof ZodError) {
			const flat = z.flattenError(err as z.ZodError<z.infer<typeof FormDataSchema>>);
			event?.set('actionError', flat);

			return { error: flat };
		}
	}
}

export default function Create() {
	const fetcher = useFetcher<Route.ComponentProps['actionData']>();
	const loading = fetcher.state !== 'idle';
	const fetcherError = fetcher.data?.error;
	const csrfToken = useCsrfToken();

	// Preview
	const ref = useRef<HTMLFormElement>(null);
	const [data, setData] = useState<FormData>();

	useEffect(() => {
		if (!ref.current) return;
		setData(new FormData(ref.current));
	}, []);

	return (
		<Page title='New Event'>
			<Card className='shadow-none w-md'>
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
							<input type='hidden' name='_csrf' value={csrfToken} />

							{!!fetcherError?.formErrors.length && (
								<Alert variant='destructive'>
									<AlertTitle>Validation error!</AlertTitle>
									<AlertDescription>{fetcherError.formErrors}</AlertDescription>
								</Alert>
							)}

							<FieldGroup className='flex flex-col gap-4'>
								<Field>
									<FieldLabel htmlFor='name'>Name</FieldLabel>
									<Input id='name' name='name' type='text' required />
									<FieldError>{fetcherError?.fieldErrors.name}</FieldError>
								</Field>

								<Field>
									<FieldLabel htmlFor='slug'>Slug</FieldLabel>
									<Input id='slug' name='slug' type='text' required />
									<FieldError>{fetcherError?.fieldErrors.slug}</FieldError>
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
		</Page>
	);
}
