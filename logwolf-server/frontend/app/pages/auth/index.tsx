import { redirect } from 'react-router';
import type { Route } from './+types';

import { getGitHubAuthURL, handleGitHubCallback } from '~/lib/auth.server';

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

export default function Auth({ loaderData }: Route.ComponentProps) {
	return (
		<div className='flex min-h-screen items-center justify-center'>
			<div className='flex flex-col items-center gap-6'>
				<h1 className='text-2xl font-semibold'>Sign in to Logwolf</h1>
				{loaderData.error === 'unauthorized' && (
					<p className='text-sm text-red-500'>Your GitHub account is not authorized for this instance.</p>
				)}
				<form method='post'>
					<button
						type='submit'
						className='flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-white hover:bg-gray-700'
					>
						Continue with GitHub
					</button>
				</form>
			</div>
		</div>
	);
}
