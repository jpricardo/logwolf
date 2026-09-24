import { describe, expect, it, vi } from 'vitest';

import {
	allowlistFromEnv,
	checkInvitee,
	inviteWarning,
	isAllowed,
	isEmptyAllowlist,
	listGithubOrgs,
	parseGithubLogins,
} from './allowlist.server';

const orgs = (...names: string[]) => vi.fn(async () => names);

describe('parseGithubLogins', () => {
	it('trims, lowercases, drops blanks and dedupes', () => {
		expect(parseGithubLogins(' Alice, bob,,alice , ,BOB')).toEqual(['alice', 'bob']);
	});

	it('reads unset and empty variables as no logins', () => {
		expect(parseGithubLogins()).toEqual([]);
		expect(parseGithubLogins('')).toEqual([]);
		expect(parseGithubLogins(' , ')).toEqual([]);
	});
});

describe('allowlistFromEnv', () => {
	it('reads both variables', () => {
		const allowlist = allowlistFromEnv({
			LOGWOLF_ALLOWED_GITHUB_USERS: 'alice,',
			LOGWOLF_ALLOWED_GITHUB_ORGS: 'Acme',
		});
		expect(allowlist).toEqual({ users: ['alice'], orgs: ['acme'] });
		expect(isEmptyAllowlist(allowlist)).toBe(false);
	});

	it('is empty when neither variable is set, or both are blank', () => {
		expect(isEmptyAllowlist(allowlistFromEnv({}))).toBe(true);
		expect(
			isEmptyAllowlist(allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: '', LOGWOLF_ALLOWED_GITHUB_ORGS: ' , ' })),
		).toBe(true);
	});
});

describe('isAllowed', () => {
	describe('only the orgs allowlist set', () => {
		const allowlist = allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme' });

		it('denies a user outside the allowed orgs', async () => {
			expect(await isAllowed('mallory', allowlist, orgs('evil-corp'))).toBe(false);
			expect(await isAllowed('mallory', allowlist, orgs())).toBe(false);
		});

		it('allows a member of an allowed org, whatever the casing', async () => {
			expect(await isAllowed('alice', allowlist, orgs('other', 'ACME'))).toBe(true);
		});
	});

	describe('only the users allowlist set', () => {
		const allowlist = allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: 'alice' });

		it('denies an unlisted user without looking up orgs', async () => {
			const listOrgs = orgs('acme');
			expect(await isAllowed('mallory', allowlist, listOrgs)).toBe(false);
			expect(listOrgs).not.toHaveBeenCalled();
		});

		it('allows a listed user, whatever the casing', async () => {
			expect(await isAllowed('Alice', allowlist, orgs())).toBe(true);
		});
	});

	describe('neither allowlist set', () => {
		it('denies everyone', async () => {
			for (const env of [{}, { LOGWOLF_ALLOWED_GITHUB_USERS: '', LOGWOLF_ALLOWED_GITHUB_ORGS: '' }]) {
				const listOrgs = orgs('acme');
				expect(await isAllowed('alice', allowlistFromEnv(env), listOrgs)).toBe(false);
				expect(listOrgs).not.toHaveBeenCalled();
			}
		});
	});

	describe('both allowlists set', () => {
		const allowlist = allowlistFromEnv({
			LOGWOLF_ALLOWED_GITHUB_USERS: 'alice',
			LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme',
		});

		it('allows a listed user without looking up orgs', async () => {
			const listOrgs = orgs();
			expect(await isAllowed('alice', allowlist, listOrgs)).toBe(true);
			expect(listOrgs).not.toHaveBeenCalled();
		});

		it('falls back to org membership for everyone else', async () => {
			expect(await isAllowed('bob', allowlist, orgs('acme'))).toBe(true);
			expect(await isAllowed('mallory', allowlist, orgs('evil-corp'))).toBe(false);
		});
	});

	it('propagates a failed org lookup instead of allowing', async () => {
		const allowlist = allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme' });
		const listOrgs = vi.fn(async (): Promise<string[]> => {
			throw new Error('boom');
		});
		await expect(isAllowed('mallory', allowlist, listOrgs)).rejects.toThrow('boom');
	});
});

describe('listGithubOrgs', () => {
	const respond = (status: number, body: unknown) =>
		vi.fn<typeof fetch>(async () => new Response(JSON.stringify(body), { status }));

	it('returns the org logins', async () => {
		const fetchImpl = respond(200, [{ login: 'acme' }, { login: 'other' }]);
		expect(await listGithubOrgs('token', fetchImpl)).toEqual(['acme', 'other']);

		const [url, init] = fetchImpl.mock.calls[0];
		expect(url).toBe('https://api.github.com/user/orgs?per_page=100');
		expect(init?.headers).toMatchObject({ Authorization: 'Bearer token' });
	});

	it('throws when GitHub answers with an error', async () => {
		const fetchImpl = respond(401, { message: 'Bad credentials' });
		await expect(listGithubOrgs('token', fetchImpl)).rejects.toThrow('401');
	});

	it('throws when the body is not a list', async () => {
		const fetchImpl = respond(200, { login: 'acme' });
		await expect(listGithubOrgs('token', fetchImpl)).rejects.toThrow('did not answer with a list');
	});
});

describe('checkInvitee', () => {
	// A fake GitHub: users by lowercase login, and the public members of each org.
	function github(
		users: Record<string, { login: string; type?: string }>,
		publicMembers: Record<string, string[]> = {},
	) {
		return vi.fn(async (input: RequestInfo | URL) => {
			const path = new URL(String(input)).pathname;

			const user = path.match(/^\/users\/([^/]+)$/);
			if (user) {
				const found = users[decodeURIComponent(user[1]).toLowerCase()];
				return found ? Response.json({ type: 'User', ...found }) : new Response(null, { status: 404 });
			}

			const member = path.match(/^\/orgs\/([^/]+)\/public_members\/([^/]+)$/);
			if (member) {
				const listed = publicMembers[member[1]]?.includes(member[2]);
				return new Response(null, { status: listed ? 204 : 404 });
			}

			return new Response(null, { status: 500 });
		}) as unknown as typeof fetch;
	}

	const octocat = { octocat: { login: 'Octocat' } };

	it('refuses a login GitHub does not know', async () => {
		const check = await checkInvitee(
			'nobody',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: 'nobody' }),
			github({}),
		);
		expect(check).toEqual({ kind: 'unknown' });
	});

	it('refuses an organization', async () => {
		const check = await checkInvitee(
			'acme',
			allowlistFromEnv({}),
			github({ acme: { login: 'Acme', type: 'Organization' } }),
		);
		expect(check).toEqual({ kind: 'organization', login: 'Acme' });
	});

	it('clears a user on the users allowlist, in GitHub’s casing', async () => {
		const check = await checkInvitee(
			'OCTOCAT',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: 'octocat' }),
			github(octocat),
		);
		expect(check).toEqual({ kind: 'allowed', login: 'Octocat' });
		expect(inviteWarning(check)).toBeUndefined();
	});

	it('clears a public member of an allowed org', async () => {
		const fetchImpl = github(octocat, { acme: ['Octocat'] });
		const check = await checkInvitee(
			'octocat',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_ORGS: 'other,acme' }),
			fetchImpl,
		);
		expect(check).toEqual({ kind: 'allowed', login: 'Octocat' });
	});

	it('warns about a user nothing clears, and says whether an org still might', async () => {
		const usersOnly = await checkInvitee(
			'octocat',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: 'alice' }),
			github(octocat),
		);
		expect(usersOnly).toEqual({ kind: 'not-allowlisted', login: 'Octocat', orgsAllowlisted: false });
		expect(inviteWarning(usersOnly)).toMatch(/cannot sign in until an admin adds them/);

		const withOrgs = await checkInvitee(
			'octocat',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme' }),
			github(octocat),
		);
		expect(withOrgs).toEqual({ kind: 'not-allowlisted', login: 'Octocat', orgsAllowlisted: true });
		expect(inviteWarning(withOrgs)).toMatch(/privately/);
	});

	it('does not block when GitHub cannot be asked', async () => {
		const down = vi.fn(async () => {
			throw new TypeError('fetch failed');
		}) as unknown as typeof fetch;
		const check = await checkInvitee('octocat', allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_USERS: 'alice' }), down);
		expect(check).toEqual({ kind: 'unverified', login: 'octocat' });
		expect(inviteWarning(check)).toMatch(/Could not reach GitHub/);

		const limited = vi.fn(async () => new Response(null, { status: 403 })) as unknown as typeof fetch;
		expect(await checkInvitee('octocat', allowlistFromEnv({}), limited)).toEqual({
			kind: 'unverified',
			login: 'octocat',
		});
	});

	it('keeps a hand-crafted login from reshaping the GitHub path', async () => {
		const fetchImpl = github({});
		await checkInvitee('../orgs/acme', allowlistFromEnv({}), fetchImpl);
		expect(String(vi.mocked(fetchImpl).mock.calls[0][0])).toBe('https://api.github.com/users/..%2Forgs%2Facme');
	});
});
