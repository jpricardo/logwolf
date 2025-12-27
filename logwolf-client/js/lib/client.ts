import { LogwolfEvent } from './event';

export class Logwolf {
	constructor(private readonly baseUrl: string) {}

	public async logEvent(ev: LogwolfEvent) {
		return fetch(new URL('/posts', this.baseUrl), {
			method: 'POST',
			body: ev.toJson(),
		});
	}
}
