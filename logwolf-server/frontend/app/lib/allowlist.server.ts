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

// --- Checking an invitee ---
//
// Adding a member only writes a membership; whether that person can ever sign
// in is the allowlist's call, which lives here and not in the broker. So before
// the dashboard adds someone it asks GitHub who they are, and checks them the
// way sign-in will.

/** What the dashboard learned about a login it is about to add to a project. */
export type InviteeCheck =
	/** No GitHub user by that name: refused, as it can only be a typo. */
	| { kind: 'unknown' }
	/** An organization, which can never sign in: refused. */
	| { kind: 'organization'; login: string }
	/** On the users allowlist, or a public member of an allowed org. */
	| { kind: 'allowed'; login: string }
	/**
	 * Neither. With orgs allowlisted they may still get in through an org whose
	 * membership they keep private, which GitHub does not show without a token.
	 */
	| { kind: 'not-allowlisted'; login: string; orgsAllowlisted: boolean }
	/** GitHub could not be asked, or did not answer usefully. */
	| { kind: 'unverified'; login: string };

const GITHUB_TIMEOUT_MS = 5000;

function githubGet(path: string, fetchImpl: typeof fetch) {
	return fetchImpl(`https://api.github.com${path}`, {
		headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'logwolf' },
		signal: AbortSignal.timeout(GITHUB_TIMEOUT_MS),
	});
}

/**
 * Checks `login` before it is added to a project, through GitHub's public API.
 * It needs no token: the user lookup and public org membership are open, and
 * rate-limited per server address, which invites come nowhere near.
 *
 * It never throws. When GitHub cannot say, the answer is `unverified`, and the
 * caller adds the member anyway: GitHub being down should not block an owner.
 */
export async function checkInvitee(
	login: string,
	allowlist: Allowlist,
	fetchImpl: typeof fetch = fetch,
): Promise<InviteeCheck> {
	let canonical: string;
	try {
		const res = await githubGet(`/users/${encodeURIComponent(login)}`, fetchImpl);
		if (res.status === 404) return { kind: 'unknown' };
		if (!res.ok) return { kind: 'unverified', login };

		const user: unknown = await res.json();
		if (typeof user !== 'object' || user === null || typeof (user as { login?: unknown }).login !== 'string') {
			return { kind: 'unverified', login };
		}
		canonical = (user as { login: string }).login;
		if ((user as { type?: unknown }).type === 'Organization') return { kind: 'organization', login: canonical };
	} catch {
		return { kind: 'unverified', login };
	}

	if (allowlist.users.includes(normalizeLogin(canonical))) return { kind: 'allowed', login: canonical };

	for (const org of allowlist.orgs) {
		try {
			// 204 for a public member; 404 for anyone else, private members included.
			const res = await githubGet(
				`/orgs/${encodeURIComponent(org)}/public_members/${encodeURIComponent(canonical)}`,
				fetchImpl,
			);
			if (res.status === 204) return { kind: 'allowed', login: canonical };
			if (res.status !== 404) return { kind: 'unverified', login: canonical };
		} catch {
			return { kind: 'unverified', login: canonical };
		}
	}

	return { kind: 'not-allowlisted', login: canonical, orgsAllowlisted: allowlist.orgs.length > 0 };
}

/**
 * What to tell the owner after adding someone the check could not clear, or
 * nothing when it did.
 */
export function inviteWarning(check: InviteeCheck): string | undefined {
	switch (check.kind) {
		case 'not-allowlisted':
			return check.orgsAllowlisted
				? `${check.login} is not on the users allowlist or a public member of an allowed org. They can sign in only if they belong to one of those orgs privately.`
				: `${check.login} is not on the allowlist, so they cannot sign in until an admin adds them to LOGWOLF_ALLOWED_GITHUB_USERS.`;
		case 'unverified':
			return `Could not reach GitHub to check ${check.login}. Make sure it is the right login, and that they are allowlisted.`;
		default:
			return undefined;
	}
}
