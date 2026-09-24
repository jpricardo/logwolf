// Helpers for testing route loaders and actions the way React Router calls
// them: a Request carrying a real, signed session cookie, and the broker client
// replaced by a fake. Only createApi is faked; sessions and CSRF are the real
// modules, so a test fails if either stops doing its job.

import { vi } from 'vitest';

import type { IApi, Project, ProjectRole, UserProject } from '~/lib/api';
import { commitSession, getSession } from '~/lib/session.server';

export const user = { login: 'Octocat', name: 'The Octocat', avatarUrl: 'https://example.com/octocat.png' };
export const CSRF = 'test-csrf-token';

/** The route context: the event the dashboard reports itself with is absent. */
export const context = { get: () => null } as never;

type SessionFields = { signedIn?: boolean; currentProjectID?: string };

/** A Cookie header for a session: signed in as `user`, with a CSRF token. */
export async function sessionCookie({ signedIn = true, currentProjectID }: SessionFields = {}): Promise<string> {
	const session = await getSession();
	if (signedIn) session.set('githubUser', user);
	session.set('csrfToken', CSRF);
	if (currentProjectID) session.set('currentProjectID', currentProjectID);
	return (await commitSession(session)).split(';')[0]!;
}

/** Reads back the session a response set, or null when it set none. */
export async function sessionSet(res: Response | { init?: ResponseInit | null }) {
	const headers = res instanceof Response ? res.headers : new Headers(res.init?.headers);
	const cookie = headers.get('Set-Cookie');
	return cookie ? getSession(cookie.split(';')[0]) : null;
}

export function get(path: string, cookie: string): Request {
	return new Request(`http://localhost${path}`, { headers: { Cookie: cookie } });
}

/**
 * A form POST. The CSRF token is the session's unless `csrf` says otherwise;
 * null leaves it out.
 */
export function post(
	path: string,
	fields: Record<string, string | string[]>,
	cookie: string,
	{ csrf = CSRF as string | null } = {},
): Request {
	const fd = new FormData();
	if (csrf !== null) fd.set('_csrf', csrf);
	for (const [k, v] of Object.entries(fields)) {
		for (const value of [v].flat()) fd.append(k, value);
	}
	return new Request(`http://localhost${path}`, { method: 'POST', body: fd, headers: { Cookie: cookie } });
}

/** Resolves to what `promise` threw: a redirect, or react-router's data(). */
export async function thrown(promise: Promise<unknown>): Promise<unknown> {
	try {
		await promise;
	} catch (err) {
		return err;
	}
	throw new Error('expected the call to throw');
}

/** The Location of a redirect Response. */
export function location(res: unknown): string | null {
	return res instanceof Response ? res.headers.get('Location') : null;
}

export function project(id: string, role: ProjectRole = 'owner', name = `Project ${id}`): UserProject {
	return { id, name, slug: name.toLowerCase().replaceAll(' ', '-'), created_at: '2026-01-01T00:00:00Z', role };
}

/**
 * A broker client whose every method rejects until a test stubs it, so a call
 * a test did not expect fails loudly.
 */
export function fakeApi(overrides: Partial<IApi> = {}): { [K in keyof IApi]: ReturnType<typeof vi.fn> } & IApi {
	const methods: (keyof IApi)[] = [
		'getProjects',
		'createProject',
		'updateProject',
		'deleteProject',
		'getMembers',
		'addMember',
		'updateMemberRole',
		'removeMember',
		'getKeys',
		'createKey',
		'deleteKey',
		'getRetention',
		'updateRetention',
		'getMetrics',
		'getLogs',
		'getLog',
		'createLog',
		'deleteLog',
	];
	const api = Object.fromEntries(
		methods.map((m) => [m, vi.fn(overrides[m] ?? (async () => Promise.reject(new Error(`unexpected call: ${m}`))))]),
	);
	return api as never;
}

export type { Project };
