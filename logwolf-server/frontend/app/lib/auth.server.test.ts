import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getSession } from './session.server';
import { unsealToken } from './token.server';

// auth.server reads the allowlist when it is loaded, so each test loads it
// afresh with the environment it sets.
async function loadAuth() {
	vi.resetModules();
	return import('./auth.server');
}

// GitHub, as sign-in talks to it: the code exchange, then the user.
function stubGitHub(login: string) {
	vi.stubGlobal(
		'fetch',
		vi.fn(async (input: RequestInfo | URL) => {
			const url = String(input);
			if (url.endsWith('/login/oauth/access_token')) return Response.json({ access_token: 'gho_signed_in' });
			if (url.endsWith('/user')) return Response.json({ login, name: 'Someone', avatar_url: 'https://x/a.png' });
			if (url.includes('/user/orgs')) return Response.json([]);
			return new Response(null, { status: 404 });
		}),
	);
}

describe('handleGitHubCallback', () => {
	beforeEach(() => vi.stubEnv('LOGWOLF_ALLOWED_GITHUB_USERS', 'octocat'));
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.unstubAllEnvs();
	});

	it('keeps the GitHub token in the session, sealed', async () => {
		stubGitHub('Octocat');
		const { handleGitHubCallback } = await loadAuth();

		const res = (await handleGitHubCallback('code', new Request('http://localhost/auth'))) as Response;

		expect(res.headers.get('Location')).toBe('/dashboard');
		const cookie = res.headers.get('Set-Cookie')!.split(';')[0]!;
		const session = await getSession(cookie);
		expect(session.get('githubUser')?.login).toBe('Octocat');
		expect(unsealToken(session.get('githubToken'))).toBe('gho_signed_in');

		// The cookie is signed, not encrypted: its payload is readable, the token is not.
		const payload = decodeURIComponent(cookie.split('=')[1]!);
		expect(Buffer.from(payload.split('.')[0]!, 'base64').toString('utf8')).not.toContain('gho_signed_in');
	});

	it('keeps nothing for a user the allowlist refuses', async () => {
		stubGitHub('stranger');
		const { handleGitHubCallback } = await loadAuth();

		const err = await handleGitHubCallback('code', new Request('http://localhost/auth')).catch((e: unknown) => e);

		expect((err as Response).headers.get('Location')).toBe('/auth?error=unauthorized');
		expect((err as Response).headers.get('Set-Cookie')).toBeNull();
	});
});
