import { redirect } from 'react-router';

import { allowlistFromEnv, isAllowed, isEmptyAllowlist, listGithubOrgs } from './allowlist.server';
import { commitSession, destroySession, getSession } from './session.server';

const GITHUB_CLIENT_ID = process.env.GITHUB_CLIENT_ID!;
const GITHUB_CLIENT_SECRET = process.env.GITHUB_CLIENT_SECRET!;
const ALLOWLIST = allowlistFromEnv();

if (isEmptyAllowlist(ALLOWLIST)) {
	console.error(
		'Neither LOGWOLF_ALLOWED_GITHUB_USERS nor LOGWOLF_ALLOWED_GITHUB_ORGS is set: nobody can sign in to the dashboard.',
	);
}

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

	// Deny by default: the login must be allowlisted or belong to an allowed org.
	let allowed = false;
	try {
		allowed = await isAllowed(user.login, ALLOWLIST, () => listGithubOrgs(access_token));
	} catch (err) {
		console.error('Could not check the sign-in allowlist', err);
	}
	if (!allowed) throw redirect('/auth?error=unauthorized');

	// Set session. The login keeps GitHub's casing for display; the broker
	// normalizes it before matching memberships.
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
