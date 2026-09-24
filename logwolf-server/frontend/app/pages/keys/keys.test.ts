import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createApi } from '~/lib/api';
import { context, fakeApi, get, post, sessionCookie } from '~/test/routes';

import { action, loader } from './index';

vi.mock('~/lib/api', async (importOriginal) => ({
	...(await importOriginal<typeof import('~/lib/api')>()),
	createApi: vi.fn(),
}));

const current = 'aaaaaaaaaaaaaaaaaaaaaaa1';
const other = 'bbbbbbbbbbbbbbbbbbbbbbb2';

function send(fields: Record<string, string | string[]>, cookie: string, opts?: { csrf?: string | null }) {
	return action({ request: post('/keys', fields, cookie, opts), params: {}, context } as never);
}

describe('/keys', () => {
	let api: ReturnType<typeof fakeApi>;

	beforeEach(() => {
		api = fakeApi({
			getKeys: async () => [],
			createKey: async (_p, scopes) => ({ key: 'lw_x', prefix: 'lw_x', id: 'k1', scopes }),
			deleteKey: async () => {},
		});
		vi.mocked(createApi).mockReturnValue(api);
	});

	it('lists the keys of the project in session', async () => {
		await loader({
			request: get('/keys', await sessionCookie({ currentProjectID: current })),
			params: {},
			context,
		} as never);
		expect(api.getKeys).toHaveBeenCalledWith(current);
	});

	it('has nothing to list without a project, and asks nothing', async () => {
		const res = await loader({ request: get('/keys', await sessionCookie()), params: {}, context } as never);
		expect(res).toEqual({ keys: [], noProject: true });
		expect(api.getKeys).not.toHaveBeenCalled();
	});

	// A tab left open on a project the user has since switched away from must
	// not mint or revoke keys there: the form cannot name the project.
	it('creates and revokes in the project in session, whatever the form says', async () => {
		const cookie = await sessionCookie({ currentProjectID: current });

		await send({ intent: 'create', scope: ['ingest', 'read'], project_id: other, projectId: other }, cookie);
		expect(api.createKey).toHaveBeenCalledWith(current, ['ingest', 'read']);

		await send({ intent: 'revoke', id: 'k1', project_id: other }, cookie);
		expect(api.deleteKey).toHaveBeenCalledWith(current, 'k1');
	});

	it('refuses a key with no scope', async () => {
		const res = await send({ intent: 'create' }, await sessionCookie({ currentProjectID: current }));
		expect(res).toMatchObject({ error: new Error('Pick at least one scope.') });
		expect(api.createKey).not.toHaveBeenCalled();
	});

	it('does nothing without a project in session', async () => {
		const res = await send({ intent: 'create', scope: 'ingest' }, await sessionCookie());
		expect(res).toMatchObject({ error: new Error('No project selected.') });
		expect(api.createKey).not.toHaveBeenCalled();
	});

	it('does nothing without the session’s CSRF token', async () => {
		const cookie = await sessionCookie({ currentProjectID: current });
		await send({ intent: 'revoke', id: 'k1' }, cookie, { csrf: null });
		expect(api.deleteKey).not.toHaveBeenCalled();
	});
});
