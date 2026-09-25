import { beforeEach, describe, expect, it, vi } from 'vitest';

import { createApi } from '~/lib/api';
import { context, fakeApi, get, location, project, sessionCookie, sessionSet, thrown } from '~/test/routes';

import { loader } from './layout';

vi.mock('~/lib/api', async (importOriginal) => ({
	...(await importOriginal<typeof import('~/lib/api')>()),
	createApi: vi.fn(),
}));

const alpha = 'aaaaaaaaaaaaaaaaaaaaaaa1';
const beta = 'bbbbbbbbbbbbbbbbbbbbbbb2';

function load(path: string, cookie: string) {
	return loader({ request: get(path, cookie), params: {}, context } as never);
}

describe('layout loader', () => {
	let projects: ReturnType<typeof project>[];

	beforeEach(() => {
		projects = [project(alpha), project(beta, 'member')];
		vi.mocked(createApi).mockReturnValue(fakeApi({ getProjects: async () => projects }));
	});

	it('renders with the stored project when the user still belongs to it', async () => {
		const res = (await load('/events', await sessionCookie({ currentProjectID: beta }))) as {
			data: { currentProject: { id: string }; projects: unknown[]; csrfToken: string };
		};

		expect(res.data.currentProject.id).toBe(beta);
		expect(res.data.projects).toHaveLength(2);
		expect(res.data.csrfToken).toBeTruthy();
	});

	// Deleted, or the user was removed from it: the child loaders of this request
	// already read the stale id, so the layout reloads the same URL with a fixed one.
	it('re-points a stale project at the first one left, and reloads the same URL', async () => {
		const res = await thrown(
			load('/events?page=3', await sessionCookie({ currentProjectID: 'ccccccccccccccccccccccc3' })),
		);

		expect(location(res)).toBe('/events?page=3');
		expect((await sessionSet(res as Response))?.get('currentProjectID')).toBe(alpha);
	});

	it('picks the first project for a session that has none yet', async () => {
		const res = await thrown(load('/dashboard', await sessionCookie()));

		expect(location(res)).toBe('/dashboard');
		expect((await sessionSet(res as Response))?.get('currentProjectID')).toBe(alpha);
	});

	it('sends a user with no projects to create one, clearing the stale id on the way', async () => {
		projects = [];

		const first = await thrown(load('/events', await sessionCookie({ currentProjectID: alpha })));
		expect(location(first)).toBe('/events');
		const cleared = await sessionSet(first as Response);
		expect(cleared?.get('currentProjectID')).toBeUndefined();

		const second = await thrown(load('/events', await sessionCookie()));
		expect(location(second)).toBe('/projects/new');
	});

	it('renders /projects/new for a user with no projects, instead of looping', async () => {
		projects = [];

		const res = (await load('/projects/new', await sessionCookie())) as { data: { currentProject?: unknown } };
		expect(res.data.currentProject).toBeUndefined();
	});

	it('sends a signed-out user to sign in, without asking the broker', async () => {
		const err = await thrown(load('/dashboard', await sessionCookie({ signedIn: false })));

		expect(location(err)).toBe('/auth');
		expect(createApi).not.toHaveBeenCalled();
	});
});
