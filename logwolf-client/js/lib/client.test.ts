import { Logwolf } from './client';
import { LogwolfEvent } from './event';
import type { LogwolfConfig } from './schema';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const testConfig = {
	url: 'http://test.url',
	sampleRate: 0.5, // tests will fail if this number is too low
	errorSampleRate: 1, // tests will fail if this number is too low
} satisfies LogwolfConfig;

describe('Logwolf', () => {
	beforeEach(() => {
		vi.useFakeTimers();

		mockFetch.mockReturnValue(
			new Promise((resolve) => resolve({ json: vi.fn().mockResolvedValue({ error: false, data: [] }) })),
		);
	});

	afterEach(() => {
		vi.resetAllMocks();
	});

	it('should create events correctly', async () => {
		const client = new Logwolf(testConfig);
		const ev = new LogwolfEvent({ name: 'Test', severity: 'info', tags: [], data: {} });

		await client.create(ev);

		expect(mockFetch).toHaveBeenCalledTimes(1);
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), { method: 'POST', body: ev.toJson() });
	});

	it('should get events correctly', async () => {
		const client = new Logwolf(testConfig);

		await client.getAll();

		expect(mockFetch).toHaveBeenCalledTimes(1);
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs?', testConfig.url), { method: 'GET' });
	});

	it('should delete events correctly', async () => {
		const client = new Logwolf(testConfig);

		await client.delete({ id: 'id' });

		expect(mockFetch).toHaveBeenCalledTimes(1);
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), {
			method: 'DELETE',
			body: JSON.stringify({ id: 'id' }),
		});
	});

	it("should capture every [severity='critical'] event", async () => {
		const client = new Logwolf(testConfig);
		const ev = new LogwolfEvent({ name: 'Test', severity: 'critical', tags: [], data: {} });

		await client.capture(ev);

		expect(mockFetch).toHaveBeenCalledTimes(1);
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), { method: 'POST', body: ev.toJson() });
	});

	it("should eventually capture a [severity='info'] or [severity='warning'] event", async () => {
		let passed = false;
		let count = 0;
		const limit = 100 * testConfig.sampleRate;
		const client = new Logwolf(testConfig);

		while (count < limit && passed === false) {
			try {
				const ev = new LogwolfEvent({ name: 'Test', severity: 'warning', tags: [], data: {} });
				await client.capture(ev);

				expect(mockFetch).toHaveBeenCalledTimes(1);
				expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), { method: 'POST', body: ev.toJson() });

				passed = true;
				break;
			} catch {
				count++;
				continue;
			}
		}

		if (!passed) throw new Error('test timed out');
	});

	it("should eventually capture a [severity='error'] event", async () => {
		let passed = false;
		let count = 0;
		const limit = 100 * testConfig.errorSampleRate;
		const client = new Logwolf(testConfig);

		while (count < limit && passed === false) {
			try {
				const ev = new LogwolfEvent({ name: 'Test', severity: 'error', tags: [], data: {} });
				await client.capture(ev);

				expect(mockFetch).toHaveBeenCalledTimes(1);
				expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), { method: 'POST', body: ev.toJson() });

				passed = true;
				break;
			} catch {
				count++;
				continue;
			}
		}

		if (!passed) throw new Error('test timed out');
	});
});
