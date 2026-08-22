import { Check } from 'lucide-react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card, CardContent } from '~/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
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
import { useCsrfToken } from '~/hooks/use-csrf-token';
import type { RetentionDays } from '~/lib/api';

import { type SettingsActionResult, useSuccessToast } from '../action-result';

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

const retentionOptions = Object.entries(retentionDaysMap);

type Props = { days: RetentionDays };

export function RetentionSection({ days }: Props) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<SettingsActionResult>();
	useSuccessToast(fetcher.data);

	return (
		<Section title='Data retention'>
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
							<input type='hidden' name='intent' value='retention' />

							<Field>
								<FieldLabel>Retention time</FieldLabel>

								<Select name='days' defaultValue={days.toString()}>
									<SelectTrigger className='w-full'>
										<SelectValue placeholder='Retention days' />
									</SelectTrigger>

									<SelectContent>
										<SelectGroup>
											<SelectLabel>Retention time</SelectLabel>
											{retentionOptions.map(([value, label]) => (
												<SelectItem key={value} value={value}>
													{label}
												</SelectItem>
											))}
										</SelectGroup>
									</SelectContent>
								</Select>

								<FieldDescription>Events older than this are dropped from this project.</FieldDescription>
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
		</Section>
	);
}
