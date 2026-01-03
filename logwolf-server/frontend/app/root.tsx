import { isRouteErrorResponse, Links, Meta, Outlet, Scripts, ScrollRestoration } from 'react-router';
import type { Route } from './+types/root';

import { EventsApi, type CreateEventDTO } from './api/events';

import './app.css';

// TODO - Register events via Logwolf Client lib
const logMiddleware: Route.MiddlewareFunction = async function ({ request, params, context }, next) {
	const start = performance.now();
	await next();
	const duration = performance.now() - start;

	const event: CreateEventDTO = {
		name: 'Navigation',
		severity: 'info',
		tags: ['test', 'logwolf_frontend', 'navigation', 'frontend'],
		data: JSON.stringify({ context, params, request }),
		duration: Math.floor(duration),
	};

	EventsApi.create(event).catch((err) => console.error(err));
};

export const middleware: Route.MiddlewareFunction[] = [logMiddleware];

export const links: Route.LinksFunction = () => [
	{ rel: 'preconnect', href: 'https://fonts.googleapis.com' },
	{
		rel: 'preconnect',
		href: 'https://fonts.gstatic.com',
		crossOrigin: 'anonymous',
	},
	{
		rel: 'stylesheet',
		href: 'https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap',
	},
];

export function Layout({ children }: { children: React.ReactNode }) {
	return (
		<html lang='en'>
			<head>
				<meta charSet='utf-8' />
				<meta name='viewport' content='width=device-width, initial-scale=1' />
				<Meta />
				<Links />
			</head>
			<body>
				{children}
				<ScrollRestoration />
				<Scripts />
			</body>
		</html>
	);
}

export default function App() {
	return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
	let message = 'Oops!';
	let details = 'An unexpected error occurred.';
	let stack: string | undefined;

	if (isRouteErrorResponse(error)) {
		message = error.status === 404 ? '404' : 'Error';
		details = error.status === 404 ? 'The requested page could not be found.' : error.statusText || details;
	} else if (import.meta.env.DEV && error && error instanceof Error) {
		details = error.message;
		stack = error.stack;
	}

	return (
		<main className='pt-16 p-4 container mx-auto'>
			<h1>{message}</h1>
			<p>{details}</p>
			{stack && (
				<pre className='w-full p-4 overflow-x-auto'>
					<code>{stack}</code>
				</pre>
			)}
		</main>
	);
}
