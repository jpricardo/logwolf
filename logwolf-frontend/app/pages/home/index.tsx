import type { Route } from '../../+types/root';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Logwolf' }, { name: 'description', content: 'Logwolf landing page!' }];
}

export default function Home() {
	return <div>Landing page</div>;
}
