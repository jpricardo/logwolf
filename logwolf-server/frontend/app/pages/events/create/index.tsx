import { CreateLogwolfEventDTOSchema, LogwolfEvent, type Severity } from '@logwolf/client-js';
import { Send } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { redirect, useFetcher } from 'react-router';
import z, { ZodError } from 'zod';

import { Page } from '~/components/nav/page';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import {
	Select,
	SelectContent,
	SelectGroup,
	SelectItem,
	SelectLabel,
	SelectTrigger,
	SelectValue,
} from '~/components/ui/select';
import { Spinner } from '~/components/ui/spinner';
import { Textarea } from '~/components/ui/textarea';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';
import { formatSeverity, severityMap } from '~/lib/format';
import { getCurrentProjectID } from '~/lib/session.server';

import type { Route } from './+types';
import { Preview } from './components/preview';

export function meta() {
	return [{ title: 'New Event - Logwolf' }];
}

const FormDataSchema = CreateLogwolfEventDTOSchema.pick({ name: true, severity: true, data: true }).and(
	z.object({
		tags: z.codec(z.string(), z.array(z.string()), {
			encode: (v) => v.join(','),
			decode: (v) => v.split(',').map((t) => t.trim()),
		}),
	}),
);

type CreateEventFormData = z.input<typeof FormDataSchema>;

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	try {
		const user = await requireAuth(request);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		// The event lands in the project the session is pointed at — the form has
		// no say in it, and neither does the dashboard's own API key.
		const projectId = await getCurrentProjectID(request);
		if (!projectId) return redirect('/projects/new');

		const d = FormDataSchema.decode(Object.fromEntries(fd.entries()) as CreateEventFormData);
		const res = await createApi(user.login)
			.createLog(projectId, new LogwolfEvent(d).toObject())
			.then(() => redirect('/events'));
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
		<Page
			title='New event'
			parents={[{ label: 'Events', to: '/events' }]}
			description='Send an event by hand, straight into this project. Handy for checking alerts and dashboards.'
		>
			<div className='grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,24rem)]'>
				<Card>
					<CardHeader>
						<CardTitle>Event</CardTitle>
						<CardDescription>Name, severity and tags are required; data is any JSON object.</CardDescription>
					</CardHeader>

					<CardContent>
						<fetcher.Form
							method='post'
							ref={ref}
							onChange={() => setData(new FormData(ref.current!))}
							className='flex flex-col gap-6'
						>
							<input type='hidden' name='_csrf' value={csrfToken} />

							{!!fetcherError?.formErrors.length && (
								<Alert variant='destructive'>
									<AlertTitle>Validation error</AlertTitle>
									<AlertDescription>{fetcherError.formErrors}</AlertDescription>
								</Alert>
							)}

							<FieldGroup className='grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_10rem]'>
								<Field>
									<FieldLabel htmlFor='name'>Name</FieldLabel>
									<Input
										id='name'
										name='name'
										type='text'
										placeholder='checkout.completed'
										required
										className='font-mono'
									/>
									<FieldError>{fetcherError?.fieldErrors.name}</FieldError>
								</Field>

								<Field>
									<FieldLabel htmlFor='severity'>Severity</FieldLabel>
									<Select name='severity' required>
										<SelectTrigger id='severity' className='w-full'>
											<SelectValue placeholder='Severity' />
										</SelectTrigger>

										<SelectContent>
											<SelectGroup>
												<SelectLabel>Severity</SelectLabel>
												{Object.keys(severityMap).map((s) => (
													<SelectItem key={s} value={s}>
														{formatSeverity(s as Severity)}
													</SelectItem>
												))}
											</SelectGroup>
										</SelectContent>
									</Select>
									<FieldError>{fetcherError?.fieldErrors.severity}</FieldError>
								</Field>
							</FieldGroup>

							<Field>
								<FieldLabel htmlFor='tags'>Tags</FieldLabel>
								<Input id='tags' name='tags' type='text' placeholder='payments, checkout' required />
								<FieldDescription>Separate tags with commas.</FieldDescription>
								<FieldError>{fetcherError?.fieldErrors.tags}</FieldError>
							</Field>

							<Field>
								<FieldLabel htmlFor='data'>Data</FieldLabel>
								<Textarea id='data' name='data' defaultValue='{}' required className='min-h-40 font-mono text-[13px]' />
								<FieldError>{fetcherError?.fieldErrors.data}</FieldError>
							</Field>

							<div className='flex justify-end border-t pt-5'>
								<Button type='submit' disabled={loading}>
									{loading ? <Spinner /> : <Send />}
									Send event
								</Button>
							</div>
						</fetcher.Form>
					</CardContent>
				</Card>

				<div className='flex h-fit flex-col gap-3 lg:sticky lg:top-20'>
					<div className='flex items-center justify-between'>
						<h2 className='text-sm font-semibold tracking-tight'>Preview</h2>
						<span className='font-mono text-xs text-muted-foreground'>form data</span>
					</div>
					<Preview formData={data} />
				</div>
			</div>
		</Page>
	);
}
