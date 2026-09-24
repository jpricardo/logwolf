import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createApi } from '~/lib/api';
import { context, fakeApi, get, location, post, project, sessionCookie, sessionSet, thrown } from '~/test/routes';

import { action, loader } from './index';

vi.mock('~/lib/api', async (importOriginal) => ({
	...(await importOriginal<typeof import('~/lib/api')>()),
	createApi: vi.fn(),
}));

const owned = 'aaaaaaaaaaaaaaaaaaaaaaa1';
const joined = 'bbbbbbbbbbbbbbbbbbbbbbb2';
const outside = 'ccccccccccccccccccccccc3';

function send(id: string, fields: Record<string, string>, cookie: string, opts?: { csrf?: string | null }) {
	return action({ request: post(`/projects/${id}/settings`, fields, cookie, opts), params: { id }, context } as never);
}

describe('/projects/:id/settings', () => {
	let api: ReturnType<typeof fakeApi>;
	let cookie: string;

	beforeEach(async () => {
		api = fakeApi({
			getProjects: async () => [project(owned, 'owner', 'Owned'), project(joined, 'member', 'Joined')],
			updateProject: async () => project(owned),
			updateRetention: async (_id, days) => ({ days: days as 90 }),
			addMember: async () => {},
			deleteProject: async () => {},
			getMembers: async () => [],
		});
		vi.mocked(createApi).mockReturnValue(api);
		cookie = await sessionCookie({ currentProjectID: owned });
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.unstubAllEnvs();
	});

	it('sends a project the user does not belong to back to /projects', async () => {
		const err = await thrown(
			loader({ request: get(`/projects/${outside}/settings`, cookie), params: { id: outside }, context } as never),
		);
		expect(location(err)).toBe('/projects');

		expect(location(await thrown(send(outside, { intent: 'rename', name: 'Mine now' }, cookie)))).toBe('/projects');
		expect(api.updateProject).not.toHaveBeenCalled();
	});

	it('refuses a form without the session’s CSRF token', async () => {
		const err = await thrown(send(owned, { intent: 'rename', name: 'X' }, cookie, { csrf: 'forged' }));
		expect(err).toMatchObject({ init: { status: 403 } });
		expect(api.updateProject).not.toHaveBeenCalled();
	});

	it.each<Record<string, string>>([
		{ intent: 'rename', name: 'Renamed' },
		{ intent: 'add-member', login: 'someone', role: 'member' },
		{ intent: 'change-role', login: 'someone', role: 'owner' },
		{ intent: 'remove-member', login: 'someone' },
		{ intent: 'delete', confirmation: 'Joined' },
	])('lets only an owner $intent', async (fields) => {
		expect(await send(joined, fields, cookie)).toEqual({ error: 'Only an owner can change this.' });
		for (const m of ['updateProject', 'addMember', 'updateMemberRole', 'removeMember', 'deleteProject'] as const) {
			expect(api[m]).not.toHaveBeenCalled();
		}
	});

	describe('retention', () => {
		it('lets a member raise it, but not lower it', async () => {
			api.getRetention.mockResolvedValue({ days: 90 });

			expect(await send(joined, { intent: 'retention', days: '30' }, cookie)).toEqual({
				error: 'Only an owner can lower retention.',
			});
			expect(api.updateRetention).not.toHaveBeenCalled();

			expect(await send(joined, { intent: 'retention', days: '0' }, cookie)).toEqual({ success: 'Retention updated.' });
			expect(api.updateRetention).toHaveBeenCalledWith(joined, 0);
		});

		it('lets an owner lower it without looking it up first', async () => {
			expect(await send(owned, { intent: 'retention', days: '30' }, cookie)).toEqual({ success: 'Retention updated.' });
			expect(api.updateRetention).toHaveBeenCalledWith(owned, 30);
			expect(api.getRetention).not.toHaveBeenCalled();
		});
	});

	describe('add-member', () => {
		// GitHub's public API, as checkInvitee asks it: one known user, one org.
		function stubGitHub() {
			vi.stubGlobal(
				'fetch',
				vi.fn(async (input: RequestInfo | URL) => {
					const path = new URL(String(input)).pathname;
					if (path === '/users/octodog') return Response.json({ login: 'OctoDog', type: 'User' });
					if (path === '/users/acme') return Response.json({ login: 'Acme', type: 'Organization' });
					return new Response(null, { status: 404 });
				}),
			);
		}

		it('refuses a login GitHub does not know, or an organization', async () => {
			stubGitHub();

			expect(await send(owned, { intent: 'add-member', login: 'nobody', role: 'member' }, cookie)).toEqual({
				error: 'There is no GitHub user named nobody.',
			});
			expect(await send(owned, { intent: 'add-member', login: 'acme', role: 'member' }, cookie)).toMatchObject({
				error: expect.stringMatching(/organization/),
			});
			expect(api.addMember).not.toHaveBeenCalled();
		});

		it('adds someone the allowlist does not clear, under GitHub’s casing, with a warning', async () => {
			stubGitHub();
			vi.stubEnv('LOGWOLF_ALLOWED_GITHUB_USERS', 'octocat');
			vi.stubEnv('LOGWOLF_ALLOWED_GITHUB_ORGS', '');

			const res = await send(owned, { intent: 'add-member', login: 'octodog', role: 'owner' }, cookie);

			expect(api.addMember).toHaveBeenCalledWith(owned, 'OctoDog', 'owner');
			expect(res).toMatchObject({
				success: 'Added OctoDog as owner.',
				warning: expect.stringMatching(/cannot sign in/),
			});
		});

		it('adds an allowlisted user without a warning', async () => {
			stubGitHub();
			vi.stubEnv('LOGWOLF_ALLOWED_GITHUB_USERS', 'octodog');

			const res = await send(owned, { intent: 'add-member', login: 'octodog', role: 'member' }, cookie);

			expect(res).toEqual({ success: 'Added OctoDog as member.', warning: undefined });
		});
	});

	describe('delete', () => {
		it('needs the project’s exact name', async () => {
			expect(await send(owned, { intent: 'delete', confirmation: 'owned' }, cookie)).toEqual({
				error: 'The project name does not match.',
			});
			expect(api.deleteProject).not.toHaveBeenCalled();
		});

		it('deletes, and takes the project out of the session when it was the current one', async () => {
			const res = await send(owned, { intent: 'delete', confirmation: 'Owned' }, cookie);

			expect(api.deleteProject).toHaveBeenCalledWith(owned);
			expect(location(res)).toBe('/projects');
			expect((await sessionSet(res as Response))?.get('currentProjectID')).toBeUndefined();
		});
	});
});
