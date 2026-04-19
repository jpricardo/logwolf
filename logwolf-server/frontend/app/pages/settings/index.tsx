import { Check } from 'lucide-react';
import { useFetcher } from 'react-router';
import { toast } from 'sonner';

import { Page } from '~/components/nav/page';
import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Section } from '~/components/ui/section';
import {
	Select,
	SelectContent,
	SelectGroup,
	SelectItem,
	SelectLabel,
	SelectTrigger,
	SelectValue,
} from '~/components/ui/select';
import { eventContext } from '~/context';
import { useCsrfToken } from '~/hooks/use-csrf-token';
import { createApi, type RetentionDays } from '~/lib/api';
import { requireAuth } from '~/lib/auth.server';
import { validateCsrfToken } from '~/lib/csrf.server';

import type { Route } from './+types';

type RetentionDaysMap<T extends number> = {
	[P in T as `${P}`]: string;
};

const retentionDaysMap: RetentionDaysMap<RetentionDays> = {
	0: 'Forever',
	30: '30 days',
	60: '60 days',
	90: '90 days',
	180: '180 days',
	365: '365 days',
};

export async function loader({ request, context }: Route.LoaderArgs) {
	const event = context.get(eventContext);
	event?.addTag('loader');

	const user = await requireAuth(request);
	const projectId = new URL(request.url).searchParams.get('projectId') ?? '';

	const api = createApi(user.login);
	const res = await api.getRetention(projectId);
	event?.set('loaderData', res);

	return { ...res, projectId };
}

export async function action({ request, context }: Route.ActionArgs) {
	const event = context.get(eventContext);
	event?.addTag('action');

	try {
		const user = await requireAuth(request);
		const fd = await request.formData();

		await validateCsrfToken(request, fd);

		const intent = fd.get('intent');
		event?.set('intent', intent);

		if (intent === 'update') {
			const days = fd.get('days');
			const projectId = fd.get('projectId')?.toString() ?? '';
			const api = createApi(user.login);
			const res = await api.updateRetention(projectId, +days!);
			event?.set('actionData', res);
			return { data: res };
		}

		return null;
	} catch (err) {
		event?.setSeverity('error');
		event?.set('actionError', err);
		return { error: err as Error };
	}
}

export function meta() {
	return [{ title: 'Settings - Logwolf' }];
}

type FetcherData = Awaited<ReturnType<typeof action>>;

export default function Settings({ loaderData }: Route.ComponentProps) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<FetcherData>();
	const actionData = fetcher.data;

	if (actionData?.data) toast('Updated retention days: ' + retentionDaysMap[actionData.data.days]);

	return (
		<Page title='Settings'>
			<div className='flex flex-col gap-8'>
				<Section title='Settings'>
					<div className='flex flex-col gap-2'>
						<Card className='shadow-none max-w-md'>
							<CardContent className='flex flex-col py-3'>
								<fetcher.Form method='post' className='w-full'>
									<FieldGroup>
										{actionData?.error && (
											<Alert>
												<AlertTitle>{actionData.error.message}</AlertTitle>
											</Alert>
										)}

										<input type='hidden' name='_csrf' value={csrfToken} />
										<input type='hidden' name='intent' value='update' />
										<input type='hidden' name='projectId' value={loaderData.projectId} />

										<Field>
											<FieldLabel>Retention time</FieldLabel>

											<Select name='days' defaultValue={loaderData.days.toString()}>
												<SelectTrigger className='w-full'>
													<SelectValue placeholder='Retention days' />
												</SelectTrigger>

												<SelectContent>
													<SelectGroup>
														<SelectLabel>Retention time</SelectLabel>
														<SelectItem value='0'>{retentionDaysMap['0']}</SelectItem>
														<SelectItem value='30'>{retentionDaysMap['30']}</SelectItem>
														<SelectItem value='60'>{retentionDaysMap['60']}</SelectItem>
														<SelectItem value='90'>{retentionDaysMap['90']}</SelectItem>
														<SelectItem value='180'>{retentionDaysMap['180']}</SelectItem>
														<SelectItem value='365'>{retentionDaysMap['365']}</SelectItem>
													</SelectGroup>
												</SelectContent>
											</Select>
										</Field>

										<Field className='flex flex-row justify-end items-end'>
											<Button type='submit' disabled={fetcher.state !== 'idle'} className='w-fit'>
												<Check />
												Save
											</Button>
										</Field>
									</FieldGroup>
								</fetcher.Form>
							</CardContent>
						</Card>
					</div>
				</Section>
			</div>
		</Page>
	);
}
