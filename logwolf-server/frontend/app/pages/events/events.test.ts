import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createApi } from '~/lib/api';
import { context, fakeApi, get, location, post, sessionCookie, thrown } from '~/test/routes';

import { action as create } from './create/index';
import { loader as details } from './details/index';

vi.mock('~/lib/api', async (importOriginal) => ({
	...(await importOriginal<typeof import('~/lib/api')>()),
	createApi: vi.fn(),
}));

const current = 'aaaaaaaaaaaaaaaaaaaaaaa1';
const logID = '66a0000000000000000000a1';

describe('event pages', () => {
	let api: ReturnType<typeof fakeApi>;

	beforeEach(() => {
		api = fakeApi({ createLog: async () => {} });
		vi.mocked(createApi).mockReturnValue(api);
	});

	describe('/events/new', () => {
		const form = { name: 'Checkout failed', severity: 'error', tags: 'checkout, payments', data: '{"order":1}' };

		it('files the event under the project in session', async () => {
			const cookie = await sessionCookie({ currentProjectID: current });
			const res = await create({
				request: post('/events/new', { ...form, project_id: 'bbbbbbbbbbbbbbbbbbbbbbb2' }, cookie),
				params: {},
				context,
			} as never);

			expect(location(res)).toBe('/events');
			expect(api.createLog).toHaveBeenCalledWith(
				current,
				expect.objectContaining({ name: 'Checkout failed', severity: 'error', tags: ['checkout', 'payments'] }),
			);
		});

		it('sends a user with no project to create one', async () => {
			const res = await create({
				request: post('/events/new', form, await sessionCookie()),
				params: {},
				context,
			} as never);

			expect(location(res)).toBe('/projects/new');
			expect(api.createLog).not.toHaveBeenCalled();
		});

		it('answers an invalid form with its field errors, sending nothing', async () => {
			const cookie = await sessionCookie({ currentProjectID: current });
			const res = await create({
				request: post('/events/new', { ...form, severity: 'apocalyptic' }, cookie),
				params: {},
				context,
			} as never);

			expect(res).toMatchObject({ error: { fieldErrors: { severity: expect.any(Array) } } });
			expect(api.createLog).not.toHaveBeenCalled();
		});
	});

	describe('/events/:id', () => {
		function load(cookie: string) {
			return details({ request: get(`/events/${logID}`, cookie), params: { id: logID }, context } as never);
		}

		it('reads the event from the project in session', async () => {
			const stored = { id: logID, name: 'Checkout failed' };
			api.getLog.mockResolvedValue(stored);

			expect(await load(await sessionCookie({ currentProjectID: current }))).toBe(stored);
			expect(api.getLog).toHaveBeenCalledWith(current, logID);
		});

		// The broker answers 404 for another project's event; there is nothing to
		// show, so the page falls back to the list.
		it('goes back to the list when the project has no such event', async () => {
			api.getLog.mockRejectedValue(new Error('log not found'));

			expect(location(await thrown(load(await sessionCookie({ currentProjectID: current }))))).toBe('/events');
		});
	});
});
