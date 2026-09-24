import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Api } from './api';

const project = 'aaaaaaaaaaaaaaaaaaaaaaa1';

describe('Api', () => {
	let fetchMock: ReturnType<typeof vi.fn>;
	const api = new Api('http://broker/', 'internal-secret', 'Octocat');

	function answer(data: unknown, status = 200) {
		fetchMock.mockResolvedValue(Response.json({ error: status >= 400, message: 'm', data }, { status }));
	}

	/** The URL, method, headers and body of the one call made. */
	function call() {
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, init] = fetchMock.mock.calls[0]! as [string, RequestInit];
		return {
			url: String(url),
			method: init.method,
			headers: new Headers(init.headers),
			body: init.body ? JSON.parse(String(init.body)) : undefined,
		};
	}

	beforeEach(() => {
		fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
	});

	afterEach(() => vi.unstubAllGlobals());

	it('sends the internal secret and the signed-in login on every call', async () => {
		answer([]);
		await api.getProjects();

		const { headers } = call();
		expect(headers.get('X-Internal-Secret')).toBe('internal-secret');
		expect(headers.get('X-User-Login')).toBe('Octocat');
	});

	// Every project-scoped call names the project in the path; none sends it
	// in a query string or a body, where the broker no longer looks.
	it.each([
		['getKeys', () => api.getKeys(project), 'GET', `projects/${project}/keys`, undefined],
		[
			'createKey',
			() => api.createKey(project, ['ingest', 'read']),
			'POST',
			`projects/${project}/keys`,
			{ scopes: ['ingest', 'read'] },
		],
		['deleteKey', () => api.deleteKey(project, 'k1'), 'DELETE', `projects/${project}/keys/k1`, undefined],
		['getRetention', () => api.getRetention(project), 'GET', `projects/${project}/retention`, undefined],
		['updateRetention', () => api.updateRetention(project, 30), 'PATCH', `projects/${project}/retention`, { days: 30 }],
		['getMetrics', () => api.getMetrics(project), 'GET', `projects/${project}/metrics`, undefined],
		['getMembers', () => api.getMembers(project), 'GET', `projects/${project}/members`, undefined],
		[
			'addMember',
			() => api.addMember(project, 'octodog', 'member'),
			'POST',
			`projects/${project}/members`,
			{ login: 'octodog', role: 'member' },
		],
		['deleteProject', () => api.deleteProject(project), 'DELETE', `projects/${project}`, undefined],
	] as const)('%s calls %s %s', async (_name, run, method, path, body) => {
		answer(method === 'GET' && path.endsWith('keys') ? [] : {});
		await run();

		const c = call();
		expect(c.method).toBe(method);
		expect(c.url).toBe(`http://broker/${path}`);
		expect(c.body).toEqual(body);
	});

	it('asks for a page of events with its pagination', async () => {
		answer([]);
		await api.getLogs(project, { page: 2, pageSize: 50 });

		expect(call().url).toBe(`http://broker/projects/${project}/logs?page=2&pageSize=50`);
	});

	// Logins and ids come from forms and URLs; encoding them keeps a crafted
	// value from reaching another route.
	it('encodes the values it puts in a path', async () => {
		answer(null);
		await api.removeMember(project, '../../keys');
		expect(call().url).toBe(`http://broker/projects/${project}/members/..%2F..%2Fkeys`);

		fetchMock.mockClear();
		answer(null);
		await api.deleteKey(project, 'k1/../../x');
		expect(call().url).toBe(`http://broker/projects/${project}/keys/k1%2F..%2F..%2Fx`);
	});

	it('throws the broker’s message when it answers an error', async () => {
		fetchMock.mockResolvedValue(
			Response.json({ error: true, message: 'only an owner can lower retention' }, { status: 403 }),
		);

		await expect(api.updateRetention(project, 30)).rejects.toThrow('only an owner can lower retention');
	});
});
