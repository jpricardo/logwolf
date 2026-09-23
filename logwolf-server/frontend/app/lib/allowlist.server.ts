// Who may sign in to the dashboard. A login gets in only if it is in the users
// allowlist or belongs to an allowed org; with both lists empty, nobody does.

export type Allowlist = {
	users: readonly string[];
	orgs: readonly string[];
};

// GitHub logins and org names are case-insensitive, and GitHub returns its own
// casing at sign-in, so both sides of the allowlist check are compared lowercase.
export const normalizeLogin = (s: string) => s.trim().toLowerCase();

/**
 * Splits a comma-separated list of GitHub logins or org names the way the
 * logger's `data.ParseGithubLogins` does: trimmed, lowercased, blanks dropped,
 * duplicates removed.
 */
export function parseGithubLogins(raw?: string): string[] {
	const logins = (raw ?? '').split(',').map(normalizeLogin);
	return [...new Set(logins.filter(Boolean))];
}

export function allowlistFromEnv(env: Record<string, string | undefined> = process.env): Allowlist {
	return {
		users: parseGithubLogins(env.LOGWOLF_ALLOWED_GITHUB_USERS),
		orgs: parseGithubLogins(env.LOGWOLF_ALLOWED_GITHUB_ORGS),
	};
}

export const isEmptyAllowlist = (allowlist: Allowlist) => allowlist.users.length === 0 && allowlist.orgs.length === 0;

/**
 * Reports whether `login` may sign in. `listOrgs` returns the orgs the user
 * belongs to and is only called when the answer depends on it.
 */
export async function isAllowed(
	login: string,
	allowlist: Allowlist,
	listOrgs: () => Promise<string[]>,
): Promise<boolean> {
	if (allowlist.users.includes(normalizeLogin(login))) return true;
	if (allowlist.orgs.length === 0) return false;

	const orgs = await listOrgs();
	return orgs.some((org) => allowlist.orgs.includes(normalizeLogin(org)));
}

/**
 * Lists the orgs of the user behind `accessToken`. Throws when GitHub does not
 * answer with a list, so an error body is never mistaken for "no orgs" or,
 * worse, read as a membership.
 */
export async function listGithubOrgs(accessToken: string, fetchImpl: typeof fetch = fetch): Promise<string[]> {
	const res = await fetchImpl('https://api.github.com/user/orgs?per_page=100', {
		headers: { Authorization: `Bearer ${accessToken}`, Accept: 'application/json' },
	});
	if (!res.ok) throw new Error(`GitHub /user/orgs answered ${res.status}`);

	const body: unknown = await res.json();
	if (!Array.isArray(body)) throw new Error('GitHub /user/orgs did not answer with a list');
	return body.flatMap((org) => (typeof org?.login === 'string' ? [org.login] : []));
}
