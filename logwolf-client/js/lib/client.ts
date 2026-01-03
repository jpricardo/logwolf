import z from 'zod';

import type { LogwolfEvent } from './event';
import {
	DeleteLogwolfEventDTOSchema,
	LogwolfEventSchema,
	type DeleteLogwolfEventDTO,
	type LogwolfApiResponse,
} from './schema';

export class Logwolf {
	constructor(private readonly apiUrl: string) {}

	private handleResponse<T>(r: LogwolfApiResponse<T>): T {
		if (r.error) throw new Error(r.message);
		return r.data;
	}

	public async create(p: LogwolfEvent) {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, {
			method: 'POST',
			body: p.toJson(),
		})
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}

	public async getAll() {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'GET' })
			.then<LogwolfApiResponse<Event[]>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return z.array(LogwolfEventSchema).parse(res);
	}

	public async getOne(id: string) {
		const res = this.getAll().then((r) => {
			return r.find((i) => i.id === id);
		});

		return LogwolfEventSchema.parse(res);
	}

	public async getRelated(id: string, amt: number) {
		// TODO - "relatedness" algorithm
		const res = this.getAll().then((r) => r.filter((i) => i.id !== id).slice(0, amt));
		return z.array(LogwolfEventSchema).parse(res);
	}

	public async delete(dto: DeleteLogwolfEventDTO) {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'DELETE', body: JSON.stringify(DeleteLogwolfEventDTOSchema.parse(dto)) })
			.then<LogwolfApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}
}
