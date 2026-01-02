import { Check } from 'lucide-react';
import type { FormEventHandler } from 'react';
import { type FetcherWithComponents } from 'react-router';

import { Button } from '~/components/ui/button';
import { Dialog, DialogContent } from '~/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';

type Props = {
	fetcher: FetcherWithComponents<unknown>;
	open: boolean;
	onOpenChange: (v: boolean) => void;
	onSubmit: FormEventHandler<HTMLFormElement>;
};

export function NewEvent({ fetcher, open, onOpenChange, onSubmit }: Props) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<fetcher.Form method='post' onSubmit={onSubmit}>
					<FieldGroup>
						<Field>
							<FieldLabel htmlFor='name'>Name</FieldLabel>
							<Input id='name' name='name' type='text' required />
						</Field>

						<Field>
							<FieldLabel htmlFor='severity'>Severity</FieldLabel>
							<Input id='severity' name='severity' type='text' required />
						</Field>

						<Field>
							<FieldLabel htmlFor='tags'>Tags</FieldLabel>
							<Input id='tags' name='tags' type='text' required />
						</Field>

						<Field orientation='horizontal' className='justify-end'>
							<Button type='submit'>
								<Check />
								Submit
							</Button>
						</Field>
					</FieldGroup>
				</fetcher.Form>
			</DialogContent>
		</Dialog>
	);
}
