import type { CreateEventDTO } from '~/api/events';
import { JSONBlock } from '~/components/ui/json-block';

function getData(formData: FormData): Partial<CreateEventDTO> {
	return Object.fromEntries(formData.entries());
}

type Props = {
	formData: FormData | undefined;
};

export function Preview({ formData }: Props) {
	return <JSONBlock data={formData ? getData(formData) : {}} />;
}
