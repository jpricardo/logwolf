import z from 'zod';

import type { LogwolfEvent } from './event';
import {
	DeleteLogwolfEventDTOSchema,
	LogwolfConfigSchema,
	LogwolfEventSchema,
	PaginationSchema,
	type DeleteLogwolfEventDTO,
	type LogwolfApiResponse,
	type LogwolfConfig,
	type LogwolfEventData,
	type Pagination,
} from './schema';

export class Logwolf {
	private readonly config: LogwolfConfig;
	private readonly baseUrl: URL;

	// Queue holds stopped LogwolfEvent instances ready to be flushed.
	private queue: LogwolfEvent[] = [];
	private flushTimer: ReturnType<typeof setInterval> | null = null;
	private isFlushing = false;

	constructor(config: LogwolfConfig) {
		this.config = LogwolfConfigSchema.parse(config);
		// Pre-compute base URL once to avoid repeated allocations per request.
		this.baseUrl = new URL(this.config.url);
	}

	// --- Private: helpers ---

	private shouldCapture(event: LogwolfEvent): boolean {
		switch (event.severity) {
			case 'error':
				return this.config.errorSampleRate === undefined || event.random >= 1 - this.config.errorSampleRate;
			case 'critical':
				return true;
			default:
				return this.config.sampleRate === undefined || event.random >= 1 - this.config.sampleRate;
		}
	}

	private getHeaders(): HeadersInit {
		return {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${this.config.apiKey}`,
		};
	}

	private handleResponse<T>(r: LogwolfApiResponse<T>): T {
		if (r.error) throw new Error(r.message);
		return r.data;
	}

	private sleep(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	// --- Public API ---

	/**
	 * Enqueues an event for batched delivery. Respects sampleRate and
	 * errorSampleRate. Returns true if the event was accepted into the queue,
	 * false if it was dropped (either by sampling or queue cap).
	 *
	 * capture() is synchronous and returns immediately — delivery happens in
	 * the background. Call flush() before process exit to drain the queue.
	 */
	public capture(event: LogwolfEvent): boolean {
		if (!this.shouldCapture(event)) return false;
		return this.enqueue(event);
	}

	/**
	 * Sends an event immediately, bypassing sampling and the queue.
	 * Awaitable — resolves when the server has accepted the event.
	 */
	public async create(event: LogwolfEvent): Promise<void> {
		event.stop();
		const url = new URL('/logs', this.baseUrl);
		const res = await fetch(url, {
			method: 'POST',
			headers: this.getHeaders(),
			body: JSON.stringify(event.toObject()),
		})
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}

	public async getAll(p?: Pagination): Promise<LogwolfEventData[]> {
		const params = p ? PaginationSchema.encode(p) : '';
		const url = new URL('/logs?' + params, this.baseUrl);
		const res = await fetch(url, { method: 'GET', headers: this.getHeaders() })
			.then<LogwolfApiResponse<Event[]>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return z.array(LogwolfEventSchema).parse(res);
	}

	public async getOne(id: string): Promise<LogwolfEventData | undefined> {
		return this.getAll().then((r) => r.find((i) => i.id === id));
	}

	public async delete(dto: DeleteLogwolfEventDTO): Promise<void> {
		const url = new URL('/logs', this.baseUrl);
		const res = await fetch(url, {
			method: 'DELETE',
			headers: this.getHeaders(),
			body: JSON.stringify(DeleteLogwolfEventDTOSchema.parse(dto)),
		})
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}

	/**
	 * Flushes all queued events immediately. Call this before process exit
	 * or page unload to avoid losing buffered events.
	 */
	public async flush(): Promise<void> {
		if (this.isFlushing || this.queue.length === 0) return;
		await this.flushQueue();
	}

	/**
	 * Clears the flush interval and prevents further background flushing.
	 * Call this to allow Node.js to exit cleanly, or in test teardown.
	 * Any queued events that haven't been flushed will be lost — call
	 * flush() first if you need to drain the queue.
	 */
	public destroy(): void {
		if (this.flushTimer !== null) {
			clearInterval(this.flushTimer);
			this.flushTimer = null;
		}
	}

	// --- Private: queue management ---

	private enqueue(event: LogwolfEvent): boolean {
		// Stop the stopwatch at enqueue time — this is the correct moment.
		event.stop();

		// Enforce the queue cap: drop the oldest event to make room.
		if (this.queue.length >= this.config.maxQueueSize) {
			const dropped = this.queue.splice(0, 1);
			this.config.onDropped?.(dropped, 'queue_full');
		}

		this.queue.push(event);

		// Start the flush timer lazily on first enqueue.
		if (this.flushTimer === null) {
			this.flushTimer = setInterval(() => {
				this.flushQueue().catch(() => {
					// Errors are handled inside flushQueue; this prevents
					// unhandled promise rejection from the interval callback.
				});
			}, this.config.flushIntervalMs);
		}

		// Flush immediately if the batch size threshold is reached.
		if (this.queue.length >= this.config.maxBatchSize) {
			this.flushQueue().catch(() => {});
		}

		return true;
	}

	private async flushQueue(): Promise<void> {
		if (this.isFlushing || this.queue.length === 0) return;

		this.isFlushing = true;

		// Drain the queue into a local batch atomically. Any events captured
		// during the flush go into the next batch.
		const batch = this.queue.splice(0, this.queue.length);

		try {
			await this.sendBatchWithRetry(batch);
		} catch {
			// All retries exhausted — notify caller via onDropped.
			this.config.onDropped?.(batch, 'send_failed');
		} finally {
			this.isFlushing = false;
		}
	}

	private async sendBatchWithRetry(batch: LogwolfEvent[]): Promise<void> {
		const url = new URL('/logs/batch', this.baseUrl);
		const body = JSON.stringify(batch.map((ev) => ev.toObject()));

		let lastError: unknown;

		for (let attempt = 0; attempt <= this.config.retryDelaysMs.length; attempt++) {
			try {
				const response = await fetch(url, {
					method: 'POST',
					headers: this.getHeaders(),
					body,
				});

				// 401/403 — bad key, do not retry, surface immediately.
				if (response.status === 401 || response.status === 403) {
					this.config.onDropped?.(batch, `auth_error_${response.status}`);
					return;
				}

				if (response.ok) return;

				// Non-2xx that isn't an auth error — retry.
				lastError = new Error(`Server returned ${response.status}`);
			} catch (err) {
				// Network error — retry.
				lastError = err;
			}

			// Wait before the next attempt, unless this was the last one.
			if (attempt < this.config.retryDelaysMs.length) {
				await this.sleep(this.config.retryDelaysMs[attempt]!);
			}
		}

		throw lastError;
	}
}
