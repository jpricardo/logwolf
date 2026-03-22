import { Logwolf } from './client';
import { LogwolfEvent } from './event';
import type { LogwolfConfig } from './schema';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const testConfig = {
	url: 'http://test.url',
	apiKey: 'lw_testkey123456789',
	sampleRate: 0.5,
	errorSampleRate: 1,

	flushIntervalMs: 2000,
	maxBatchSize: 20,
	maxQueueSize: 500,
	retryDelaysMs: [0, 0, 0],
} satisfies LogwolfConfig;

const immediateConfig = {
	...testConfig,
	flushIntervalMs: 50,
	maxBatchSize: 3,
	maxQueueSize: 5,
} satisfies LogwolfConfig;

function makeEvent(severity: LogwolfEvent['severity'] = 'info') {
	return new LogwolfEvent({ name: 'Test', severity, tags: [], data: {} });
}

function okResponse() {
	return Promise.resolve(
		new Response(JSON.stringify({ error: false, data: undefined, message: 'OK' }), { status: 200 }),
	);
}

/**
 * Runs a promise to completion while repeatedly advancing fake timers.
 * Needed because retry delays use setTimeout under fake timers — without
 * pumping, any sleep() call will hang forever.
 */
async function drainTimers(promise: Promise<unknown>, { step = 1, max = 20 } = {}): Promise<void> {
	let done = false;
	promise.finally(() => {
		done = true;
	});
	for (let i = 0; i < max && !done; i++) {
		await vi.advanceTimersByTimeAsync(step);
	}
}

describe('Logwolf', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockFetch.mockReturnValue(okResponse());
	});

	afterEach(() => {
		vi.resetAllMocks();
		vi.useRealTimers();
	});

	// --- create() ---

	describe('create()', () => {
		it('sends immediately, bypassing the queue', async () => {
			const client = new Logwolf(testConfig);
			const ev = makeEvent();

			await client.create(ev);

			expect(mockFetch).toHaveBeenCalledTimes(1);
			expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: 'Bearer lw_testkey123456789' },
				body: JSON.stringify(ev.toObject()),
			});
		});

		it('stops the event stopwatch before sending', async () => {
			const client = new Logwolf(testConfig);
			const ev = makeEvent();

			vi.advanceTimersByTime(100);
			await client.create(ev);

			const body = JSON.parse(mockFetch.mock.calls.at(0)?.at(1).body);
			expect(body.duration).toBeGreaterThanOrEqual(0);
		});
	});

	// --- capture() ---

	describe('capture()', () => {
		it('returns true when the event is accepted into the queue', () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 1 });
			const ev = makeEvent();
			expect(client.capture(ev)).toBe(true);
			client.destroy();
		});

		it('returns false when the event is dropped by sampling', () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 0.0001 });
			const ev = Object.assign(makeEvent('info'), { random: 0 });
			expect(client.capture(ev)).toBe(false);
			client.destroy();
		});

		it('always captures critical severity regardless of sampleRate', () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 0.0001 });
			const ev = makeEvent('critical');
			expect(client.capture(ev)).toBe(true);
			client.destroy();
		});

		it('stops the event stopwatch at enqueue time, not flush time', async () => {
			const client = new Logwolf({ ...immediateConfig, sampleRate: 1, maxBatchSize: 10 });
			const ev = makeEvent();

			client.capture(ev);

			// Flush fires at flushIntervalMs (50ms). Advance past it.
			await vi.advanceTimersByTimeAsync(100);

			const body = JSON.parse(mockFetch.mock.calls.at(0)?.at(1).body);
			// Duration should reflect time up to capture(), not up to flush.
			// It will be close to 0 since no real time passed before capture().
			expect(body[0].duration).toBeLessThan(10);
			client.destroy();
		});

		it('does not start the flush timer until first capture()', () => {
			const client = new Logwolf(immediateConfig);

			vi.advanceTimersByTime(immediateConfig.flushIntervalMs! * 3);
			expect(mockFetch).not.toHaveBeenCalled();

			client.destroy();
		});

		it('drops the oldest event and calls onDropped when queue is full', () => {
			const dropped: LogwolfEvent[] = [];
			const client = new Logwolf({
				...immediateConfig,
				sampleRate: 1,
				maxQueueSize: 3,
				maxBatchSize: 100,
				onDropped: (events) => dropped.push(...events),
			});

			const ev1 = makeEvent();
			const ev2 = makeEvent();
			const ev3 = makeEvent();
			const ev4 = makeEvent();

			client.capture(ev1);
			client.capture(ev2);
			client.capture(ev3);
			// Queue is full — ev1 should be dropped to make room for ev4
			client.capture(ev4);

			expect(dropped).toHaveLength(1);
			expect(dropped[0]).toBe(ev1);

			client.destroy();
		});
	});

	// --- flush() ---

	describe('flush()', () => {
		it('sends all queued events as a batch', async () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 1, maxBatchSize: 100, retryDelaysMs: [] });

			client.capture(makeEvent());
			client.capture(makeEvent());
			client.capture(makeEvent());

			await drainTimers(client.flush());

			expect(mockFetch).toHaveBeenCalledTimes(1);
			expect(mockFetch).toHaveBeenCalledWith(
				new URL('/logs/batch', testConfig.url),
				expect.objectContaining({ method: 'POST' }),
			);
			const body = JSON.parse(mockFetch.mock.calls.at(0)?.at(1).body);
			expect(body).toHaveLength(3);

			client.destroy();
		});

		it('does nothing when the queue is empty', async () => {
			const client = new Logwolf(testConfig);
			await drainTimers(client.flush());
			expect(mockFetch).not.toHaveBeenCalled();
			client.destroy();
		});

		it('flushes automatically when maxBatchSize is reached', async () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 1, maxBatchSize: 2, retryDelaysMs: [] });

			client.capture(makeEvent());
			client.capture(makeEvent());

			// The auto-flush is triggered synchronously inside enqueue() but
			// the fetch itself is async — let microtasks settle.
			await vi.advanceTimersByTimeAsync(0);

			expect(mockFetch).toHaveBeenCalledTimes(1);
			const body = JSON.parse(mockFetch.mock.calls.at(0)?.at(1).body);
			expect(body).toHaveLength(2);

			client.destroy();
		});

		it('flushes automatically on the interval', async () => {
			const client = new Logwolf({
				...testConfig,
				sampleRate: 1,
				flushIntervalMs: 100,
				maxBatchSize: 100,
				retryDelaysMs: [],
			});

			client.capture(makeEvent());

			await vi.advanceTimersByTimeAsync(100);

			expect(mockFetch).toHaveBeenCalledTimes(1);
			client.destroy();
		});
	});

	// --- retry & onDropped ---

	describe('retry and onDropped', () => {
		it('retries on server error and succeeds on second attempt', async () => {
			mockFetch
				.mockResolvedValueOnce(new Response('error', { status: 500 }))
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ error: false, data: null, message: 'OK' }), { status: 200 }),
				);

			const client = new Logwolf({ ...testConfig, sampleRate: 1, maxBatchSize: 100, retryDelaysMs: [0, 0, 0] });

			client.capture(makeEvent());
			await drainTimers(client.flush());

			expect(mockFetch).toHaveBeenCalledTimes(2);
			client.destroy();
		});

		it('calls onDropped after all retries are exhausted', async () => {
			mockFetch.mockResolvedValue(new Response('error', { status: 500 }));

			const dropped: LogwolfEvent[] = [];
			const droppedReasons: string[] = [];

			const client = new Logwolf({
				...testConfig,
				sampleRate: 1,
				maxBatchSize: 100,
				retryDelaysMs: [0, 0, 0],
				onDropped: (events, reason) => {
					dropped.push(...events);
					droppedReasons.push(reason);
				},
			});

			client.capture(makeEvent());
			await drainTimers(client.flush());

			// 1 initial attempt + 3 retries = 4 total fetches
			expect(mockFetch).toHaveBeenCalledTimes(4);
			expect(dropped).toHaveLength(1);
			expect(droppedReasons[0]).toBe('send_failed');
			client.destroy();
		});

		it('does not retry on 401 and calls onDropped immediately', async () => {
			mockFetch.mockResolvedValue(new Response('unauthorized', { status: 401 }));

			const droppedReasons: string[] = [];
			const client = new Logwolf({
				...testConfig,
				sampleRate: 1,
				maxBatchSize: 100,
				retryDelaysMs: [0, 0, 0],
				onDropped: (_, reason) => droppedReasons.push(reason),
			});

			client.capture(makeEvent());
			await drainTimers(client.flush());

			expect(mockFetch).toHaveBeenCalledTimes(1);
			expect(droppedReasons[0]).toBe('auth_error_401');
			client.destroy();
		});
	});

	// --- destroy() ---

	describe('destroy()', () => {
		it('stops the flush interval', async () => {
			const client = new Logwolf({ ...testConfig, sampleRate: 1, flushIntervalMs: 100, maxBatchSize: 100 });

			client.capture(makeEvent());
			client.destroy();

			await vi.advanceTimersByTimeAsync(1000);

			expect(mockFetch).not.toHaveBeenCalled();
		});
	});

	// --- getAll / delete (unchanged) ---

	it('gets events correctly', async () => {
		mockFetch.mockReturnValue(Promise.resolve({ json: vi.fn().mockResolvedValue({ error: false, data: [] }) }));
		const client = new Logwolf(testConfig);
		await client.getAll();

		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs?', testConfig.url), {
			method: 'GET',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer lw_testkey123456789' },
		});
	});

	it('deletes events correctly', async () => {
		mockFetch.mockReturnValue(Promise.resolve({ json: vi.fn().mockResolvedValue({ error: false, data: undefined }) }));
		const client = new Logwolf(testConfig);
		await client.delete({ id: 'id' });

		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testConfig.url), {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer lw_testkey123456789' },
			body: JSON.stringify({ id: 'id' }),
		});
	});
});
