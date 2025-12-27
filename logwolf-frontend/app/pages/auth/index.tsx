import type { Route } from '../../+types/root';

export function meta({}: Route.MetaArgs) {
	return [{ title: 'Auth - Logwolf' }, { name: 'description', content: 'Logwolf auth form!' }];
}

export default function Auth() {
	return <div>Auth</div>;
}
