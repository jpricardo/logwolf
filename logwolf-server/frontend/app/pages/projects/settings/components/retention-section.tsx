import { Check } from 'lucide-react';
import { useFetcher } from 'react-router';

import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
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
import { lowersRetention } from '~/lib/retention';

import { type SettingsActionResult, useSuccessToast } from '../action-result';
import { SettingsFooter, SettingsRow } from './settings-row';

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

type Props = {
	days: RetentionDays;
	/** Owners only: a member may keep logs longer, never shorter. */
	canLower: boolean;
};

export function RetentionSection({ days, canLower }: Props) {
	const csrfToken = useCsrfToken();
	const fetcher = useFetcher<SettingsActionResult>();
	useSuccessToast(fetcher.data);

	return (
		<SettingsRow
			title='Data retention'
			description='Events older than this are dropped from this project on the next cleanup pass.'
		>
			<Card className='gap-0 overflow-hidden py-0'>
				<fetcher.Form method='post'>
					<FieldGroup className='p-5'>
						{fetcher.data?.error && (
							<Alert variant='destructive'>
								<AlertTitle>{fetcher.data.error}</AlertTitle>
							</Alert>
						)}

						<input type='hidden' name='_csrf' value={csrfToken} />
						<input type='hidden' name='intent' value='retention' />

						<Field>
							<FieldLabel htmlFor='retention-days'>Retention time</FieldLabel>

							<Select name='days' defaultValue={days.toString()}>
								<SelectTrigger id='retention-days' className='w-full max-w-sm'>
									<SelectValue placeholder='Retention days' />
								</SelectTrigger>

								<SelectContent>
									<SelectGroup>
										<SelectLabel>Retention time</SelectLabel>
										{retentionOptions.map(([value, label]) => (
											<SelectItem
												key={value}
												value={value}
												disabled={!canLower && lowersRetention(days, Number(value))}
											>
												{label}
											</SelectItem>
										))}
									</SelectGroup>
								</SelectContent>
							</Select>
						</Field>
					</FieldGroup>

					<SettingsFooter hint={canLower ? undefined : 'Only an owner can shorten it.'}>
						<Button type='submit' size='sm' disabled={fetcher.state !== 'idle'}>
							<Check />
							Save
						</Button>
					</SettingsFooter>
				</fetcher.Form>
			</Card>
		</SettingsRow>
	);
}
