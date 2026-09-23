import { describe, expect, it, vi } from 'vitest';

import { allowlistFromEnv, isAllowed, isEmptyAllowlist, listGithubOrgs, parseGithubLogins } from './allowlist.server';

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
