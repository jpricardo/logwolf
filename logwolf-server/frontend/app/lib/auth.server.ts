import { redirect } from 'react-router';

import { commitSession, destroySession, getSession } from './session.server';

const GITHUB_CLIENT_ID = process.env.GITHUB_CLIENT_ID!;
const GITHUB_CLIENT_SECRET = process.env.GITHUB_CLIENT_SECRET!;
const ALLOWED_GITHUB_USERS = process.env.LOGWOLF_ALLOWED_GITHUB_USERS?.split(',').map((s) => s.trim()) ?? [];
const ALLOWED_GITHUB_ORGS = process.env.LOGWOLF_ALLOWED_GITHUB_ORGS?.split(',').map((s) => s.trim()) ?? [];

export function getGitHubAuthURL() {
	const params = new URLSearchParams({
		client_id: GITHUB_CLIENT_ID,
		scope: 'read:user read:org',
	});
	return `https://github.com/login/oauth/authorize?${params}`;
}

export async function handleGitHubCallback(code: string, request: Request) {
	// Exchange code for token
	const tokenRes = await fetch('https://github.com/login/oauth/access_token', {
		method: 'POST',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ client_id: GITHUB_CLIENT_ID, client_secret: GITHUB_CLIENT_SECRET, code }),
	});
	const { access_token } = await tokenRes.json();

	// Fetch user
	const userRes = await fetch('https://api.github.com/user', {
		headers: { Authorization: `Bearer ${access_token}`, Accept: 'application/json' },
	});
	const user = await userRes.json();

	// Allow-list check by username
	if (ALLOWED_GITHUB_USERS.length > 0 && !ALLOWED_GITHUB_USERS.includes(user.login)) {
		// Check org membership as fallback
		if (ALLOWED_GITHUB_ORGS.length > 0) {
			const orgsRes = await fetch('https://api.github.com/user/orgs', {
				headers: { Authorization: `Bearer ${access_token}` },
			});
			const orgs: { login: string }[] = await orgsRes.json();
			const memberOfAllowedOrg = orgs.some((o) => ALLOWED_GITHUB_ORGS.includes(o.login));
			if (!memberOfAllowedOrg) throw redirect('/auth?error=unauthorized');
		} else {
			throw redirect('/auth?error=unauthorized');
		}
	}

	// Set session
	const session = await getSession(request.headers.get('Cookie'));
	session.set('githubUser', {
		login: user.login,
		name: user.name,
		avatarUrl: user.avatar_url,
	});

	return redirect('/dashboard', {
		headers: { 'Set-Cookie': await commitSession(session) },
	});
}

export async function requireAuth(request: Request) {
	const session = await getSession(request.headers.get('Cookie'));
	const user = session.get('githubUser');
	if (!user) throw redirect('/auth');
	return user;
}

export async function logout(request: Request) {
	const session = await getSession(request.headers.get('Cookie'));
	return redirect('/', {
		headers: { 'Set-Cookie': await destroySession(session) },
	});
}
