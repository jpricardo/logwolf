import { Page } from '~/components/nav/page';
import { Api } from '~/lib/api';
import type { Route } from '../../+types/root';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Dashboard - Logwolf' }, { name: 'description', content: 'Logwolf dashboard!' }];
}

export async function loader({ params }: Route.ClientLoaderArgs) {
	const url = new URL('/logs', 'http://localhost:8080/');
	const res = await fetch(url, { method: 'GET' }).then((r) => r.json());
	return Api.getLogs();
}

export default function Dashboard({ loaderData }: Route.ComponentProps) {
	console.log(loaderData);

	return (
		<Page title='Dashboard'>
			<div>Dashboard</div>
		</Page>
	);
}
