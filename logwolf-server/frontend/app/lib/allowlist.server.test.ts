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
	// A fake GitHub: users by lowercase login, the public members of each org,
	// and every member, public or private, which the members endpoint shows
	// only to a token of someone in that org, as GitHub's does. It answers the
	// way the real API did when asked: 204 or 404 to a member's token, 302 to a
	// token from outside the org, 401 to a bad one.
	function github(
		users: Record<string, { login: string; type?: string }>,
		publicMembers: Record<string, string[]> = {},
		allMembers: Record<string, string[]> = {},
		tokens: Record<string, string> = {}, // token -> login it belongs to
	) {
		return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
			const path = new URL(String(input)).pathname;

			const members = path.match(/^\/orgs\/([^/]+)\/members\/([^/]+)$/);
			if (members) {
				const auth = new Headers(init?.headers).get('Authorization')?.replace('Bearer ', '');
				const owner = auth ? tokens[auth] : undefined;
				if (auth && !owner) return new Response(null, { status: 401 });
				if (!owner || !allMembers[members[1]]?.includes(owner)) return new Response(null, { status: 302 });
				return new Response(null, { status: allMembers[members[1]]?.includes(members[2]) ? 204 : 404 });
			}

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
		expect(usersOnly).toEqual({
			kind: 'not-allowlisted',
			login: 'Octocat',
			orgsAllowlisted: false,
			privateChecked: true,
		});
		expect(inviteWarning(usersOnly)).toMatch(/cannot sign in until an admin adds them/);

		const withOrgs = await checkInvitee(
			'octocat',
			allowlistFromEnv({ LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme' }),
			github(octocat),
		);
		expect(withOrgs).toEqual({
			kind: 'not-allowlisted',
			login: 'Octocat',
			orgsAllowlisted: true,
			privateChecked: false,
		});
		expect(inviteWarning(withOrgs)).toMatch(/privately/);
	});

	describe('with the inviting owner’s token', () => {
		// alice owns the project and is in acme; octocat is in acme privately.
		const acme = { LOGWOLF_ALLOWED_GITHUB_ORGS: 'acme' };
		const members = { acme: ['alice', 'Octocat'] };
		const tokens = { 'alice-token': 'alice', 'bob-token': 'bob' };

		it('sees an allowed org’s private members, which the public check cannot', async () => {
			const fetchImpl = github(octocat, {}, members, tokens);

			expect(await checkInvitee('octocat', allowlistFromEnv(acme), fetchImpl)).toMatchObject({
				kind: 'not-allowlisted',
			});
			expect(await checkInvitee('octocat', allowlistFromEnv(acme), fetchImpl, 'alice-token')).toEqual({
				kind: 'allowed',
				login: 'Octocat',
			});
		});

		it('is certain about someone in none of the orgs, and says so', async () => {
			const check = await checkInvitee(
				'octodog',
				allowlistFromEnv(acme),
				github({ octodog: { login: 'OctoDog' } }, {}, members, tokens),
				'alice-token',
			);

			expect(check).toEqual({ kind: 'not-allowlisted', login: 'OctoDog', orgsAllowlisted: true, privateChecked: true });
			expect(inviteWarning(check)).toMatch(/cannot sign in until/);
			expect(inviteWarning(check)).not.toMatch(/privately/);
		});

		// bob is not in acme, so GitHub will not tell him about its members; and
		// a revoked token is refused. Either way the public check still runs.
		it.each(['bob-token', 'revoked-token'])('falls back to public membership for %s', async (token) => {
			const publicOnly = github(octocat, { acme: ['Octocat'] }, members, tokens);
			expect(await checkInvitee('octocat', allowlistFromEnv(acme), publicOnly, token)).toEqual({
				kind: 'allowed',
				login: 'Octocat',
			});

			const privateOnly = github(octocat, {}, members, tokens);
			const check = await checkInvitee('octocat', allowlistFromEnv(acme), privateOnly, token);
			expect(check).toMatchObject({ kind: 'not-allowlisted', privateChecked: false });
		});

		it('sends the token to GitHub’s API alone', async () => {
			const fetchImpl = github(octocat, {}, members, tokens);
			await checkInvitee('octocat', allowlistFromEnv(acme), fetchImpl, 'alice-token');

			for (const [input, init] of vi.mocked(fetchImpl).mock.calls) {
				const authorized = new Headers(init?.headers).has('Authorization');
				expect(new URL(String(input)).host).toBe('api.github.com');
				expect(authorized).toBe(new URL(String(input)).pathname.includes('/members/'));
			}
		});
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
