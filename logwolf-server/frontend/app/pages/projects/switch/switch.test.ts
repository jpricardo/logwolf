import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createApi } from '~/lib/api';
import { context, fakeApi, location, post, project, sessionCookie, sessionSet, thrown } from '~/test/routes';

import { action } from './index';

vi.mock('~/lib/api', async (importOriginal) => ({
	...(await importOriginal<typeof import('~/lib/api')>()),
	createApi: vi.fn(),
}));

const alpha = 'aaaaaaaaaaaaaaaaaaaaaaa1';
const beta = 'bbbbbbbbbbbbbbbbbbbbbbb2';

describe('/projects/switch action', () => {
	let api: ReturnType<typeof fakeApi>;

	beforeEach(() => {
		api = fakeApi({ getProjects: async () => [project(alpha), project(beta, 'member')] });
		vi.mocked(createApi).mockReturnValue(api);
	});

	async function switchTo(projectId: string, redirectTo = '/events', opts?: { csrf?: string | null }) {
		const cookie = await sessionCookie({ currentProjectID: alpha });
		return action({
			request: post('/projects/switch', { projectId, redirectTo }, cookie, opts),
			params: {},
			context,
		} as never);
	}

	it('points the session at a project the user belongs to, and goes back', async () => {
		const res = await switchTo(beta, '/events?page=2');

		expect(location(res)).toBe('/events?page=2');
		expect((await sessionSet(res as Response))?.get('currentProjectID')).toBe(beta);
		expect(createApi).toHaveBeenCalledWith('Octocat');
	});

	it('leaves the session alone for a project the user does not belong to', async () => {
		const res = await switchTo('ccccccccccccccccccccccc3');

		expect(location(res)).toBe('/events');
		expect(await sessionSet(res as Response)).toBeNull();
	});

	it.each(['https://evil.example/', '//evil.example/', '/\\evil.example/', 'events'])(
		'does not redirect off the dashboard to %s',
		async (redirectTo) => {
			expect(location(await switchTo(beta, redirectTo))).toBe('/dashboard');
		},
	);

	it('refuses a form without the session’s CSRF token', async () => {
		for (const csrf of [null, 'forged']) {
			const err = await thrown(switchTo(beta, '/events', { csrf }));
			expect(err).toMatchObject({ init: { status: 403 } });
		}
		expect(api.getProjects).not.toHaveBeenCalled();
	});

	it('sends a signed-out user to sign in', async () => {
		const cookie = await sessionCookie({ signedIn: false });
		const err = await thrown(
			action({ request: post('/projects/switch', { projectId: beta }, cookie), params: {}, context } as never),
		);
		expect(location(err)).toBe('/auth');
	});
});
