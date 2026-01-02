import type { LogPayload } from './types';

export class LogwolfEvent {
	public readonly createdAt = new Date();

	constructor(
		public name: string,
		public severity: string,
		public tags: string[] = [],
		public readonly data: Record<string, unknown>,
	) {}

	public set(key: string, value: unknown) {
		this.data[key] = value;
	}

	public get(key: string) {
		return this.data[key];
	}

	public toJson() {
		const now = new Date();
		const duration = now.getTime() - this.createdAt.getTime();

		const payload: LogPayload = {
			name: this.name,
			severity: this.severity,
			tags: this.tags,
			data: this.data,
			duration: duration,
		};

		return JSON.stringify(payload);
	}
}
