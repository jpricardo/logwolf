import { LogwolfEvent } from './event';
import { handleResponse } from './helpers';
import type { LogEvent } from './types';

export class Logwolf {
	constructor(private readonly baseUrl: string) {}

	public async logEvent(ev: LogwolfEvent): Promise<never> {
		return fetch(new URL('/posts', this.baseUrl), {
			method: 'POST',
			body: ev.toJson(),
		}).then(handleResponse<never>);
	}

	public async getEvents(): Promise<LogEvent[]> {
		return fetch(new URL('/posts', this.baseUrl), { method: 'GET' }).then(handleResponse<LogEvent[]>);
	}
}
