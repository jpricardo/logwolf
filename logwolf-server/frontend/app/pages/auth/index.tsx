import { CircleAlert } from 'lucide-react';
import { Link, redirect } from 'react-router';

import { Logo } from '~/components/nav/logo';
import { Alert, AlertTitle } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { getGitHubAuthURL, handleGitHubCallback } from '~/lib/auth.server';
import { ThemeProvider } from '~/store/theme-provider';

import type { Route } from './+types';

export function meta() {
	return [{ title: 'Sign in - Logwolf' }];
}

export async function loader({ request }: Route.LoaderArgs) {
	const url = new URL(request.url);
	const code = url.searchParams.get('code');
	const error = url.searchParams.get('error');

	if (code) {
		// GitHub redirected back with a code — complete the OAuth flow
		return handleGitHubCallback(code, request);
	}

	return { error };
}

export async function action() {
	// Redirect to GitHub to start the OAuth flow
	return redirect(getGitHubAuthURL());
}

function GitHubMark(props: React.ComponentProps<'svg'>) {
	return (
		<svg viewBox='0 0 16 16' fill='currentColor' aria-hidden {...props}>
			<path d='M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z' />
		</svg>
	);
}

export default function Auth({ loaderData }: Route.ComponentProps) {
	return (
		<ThemeProvider>
			<main className='relative flex min-h-screen items-center justify-center overflow-hidden p-6'>
				<div className='bg-grid bg-grid-centered pointer-events-none absolute inset-0' aria-hidden />

				<div className='relative flex w-full max-w-sm flex-col gap-6'>
					<Link to='/' className='self-center'>
						<Logo className='text-lg' markClassName='size-7' />
					</Link>

					<div className='flex flex-col gap-6 rounded-lg border bg-card p-8 shadow-sm shadow-black/5'>
						<div className='flex flex-col gap-1.5 text-center'>
							<h1 className='text-xl font-semibold tracking-tight'>Sign in to Logwolf</h1>
							<p className='text-sm text-muted-foreground'>Sign in with GitHub to open the dashboard.</p>
						</div>

						{loaderData.error === 'unauthorized' && (
							<Alert variant='destructive'>
								<CircleAlert />
								<AlertTitle className='line-clamp-none'>
									Your GitHub account is not authorized for this instance.
								</AlertTitle>
							</Alert>
						)}

						<form method='post'>
							<Button type='submit' size='lg' className='w-full bg-foreground text-background hover:bg-foreground/90'>
								<GitHubMark className='size-4' />
								Continue with GitHub
							</Button>
						</form>
					</div>

					<p className='text-center text-xs text-muted-foreground'>
						Only allowlisted GitHub users and organizations can sign in.
					</p>
				</div>
			</main>
		</ThemeProvider>
	);
}
