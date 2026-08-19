import { LogwolfEvent } from '@logwolf/client-js';
import { isRouteErrorResponse, Links, Meta, Outlet, Scripts, ScrollRestoration } from 'react-router';

import type { Route } from './+types/root';
import { eventContext } from './context';
import { injectRequest, injectResponse, logwolf } from './lib/logwolf';
import { DEFAULT_THEME, THEME_STORAGE_KEY } from './store/theme-provider';

import './app.css';

// Runs before first paint so the document never renders light on its way to the
// stored theme. ThemeProvider takes over once React hydrates; the two must agree
// on THEME_STORAGE_KEY and on the JSON encoding useLocalStorage writes.
const themeScript = `
(function () {
	try {
		var raw = localStorage.getItem('${THEME_STORAGE_KEY}');
		var theme = '${DEFAULT_THEME}';

		if (raw) {
			try { theme = JSON.parse(raw); } catch (e) { theme = raw; }
		}

		if (theme === 'system') {
			theme = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
		}

		document.documentElement.classList.add(theme);
	} catch (e) {}
})();
`;

const logMiddleware: Route.MiddlewareFunction = async function ({ request, params, context }, next) {
	const event = new LogwolfEvent({
		name: 'Server Event',
		severity: 'info',
		tags: ['logwolf_frontend', 'server', 'navigation'],
		data: { requestId: crypto.randomUUID() },
	});

	event.set('context', context);
	event.set('params', params);
	injectRequest(event, request);

	context.set(eventContext, event);
	const response = await next();

	injectResponse(event, response);
	logwolf.capture(event);
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
		<html lang='en' suppressHydrationWarning>
			<head>
				<meta charSet='utf-8' />
				<meta name='viewport' content='width=device-width, initial-scale=1' />
				<Meta />
				<Links />
				<script dangerouslySetInnerHTML={{ __html: themeScript }} />
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
