import z from 'zod';

import type { LogwolfEvent } from './event';
import {
	DeleteLogwolfEventDTOSchema,
	LogwolfConfigSchema,
	LogwolfEventSchema,
	type DeleteLogwolfEventDTO,
	type LogwolfApiResponse,
	type LogwolfConfig,
} from './schema';

export class Logwolf {
	private readonly config: LogwolfConfig;

	constructor(config: LogwolfConfig) {
		LogwolfConfigSchema.parse(config);
		this.config = config;
	}

	private handleResponse<T>(r: LogwolfApiResponse<T>): T {
		if (r.error) throw new Error(r.message);
		return r.data;
	}

	private shouldCapture(e: LogwolfEvent): boolean {
		switch (e.severity) {
			case 'error':
				return this.config.errorSampleRate === undefined || e.random >= 1 - this.config.errorSampleRate;
			case 'critical':
				return true;
			default:
				return this.config.sampleRate === undefined || e.random >= 1 - this.config.sampleRate;
		}
	}

	/**
	 * This bypasses `sampleRate` and `errorSampleRate`, every created event is sent to the server!
	 */
	public async create(p: LogwolfEvent) {
		const url = new URL('/logs', this.config.url);
		const res = await fetch(url, { method: 'POST', body: p.toJson() })
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}

	/**
	 * A captured event is subject to `sampleRate` and `errorSampleRate`, not all captured events are sent to the server!
	 */
	public async capture(p: LogwolfEvent) {
		if (!this.shouldCapture(p)) return;

		return await this.create(p);
	}

	public async getAll() {
		const url = new URL('/logs', this.config.url);
		const res = await fetch(url, { method: 'GET' })
			.then<LogwolfApiResponse<Event[]>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return z.array(LogwolfEventSchema).parse(res);
	}

	public async getOne(id: string) {
		const res = this.getAll().then((r) => {
			return r.find((i) => i.id === id);
		});

		return res;
	}

	public async getRelated(id: string, amt: number) {
		// TODO - "relatedness" algorithm
		const res = this.getAll().then((r) => r.filter((i) => i.id !== id).slice(0, amt));
		return res;
	}

	public async delete(dto: DeleteLogwolfEventDTO) {
		const url = new URL('/logs', this.config.url);
		const res = await fetch(url, { method: 'DELETE', body: JSON.stringify(DeleteLogwolfEventDTOSchema.parse(dto)) })
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}
}
