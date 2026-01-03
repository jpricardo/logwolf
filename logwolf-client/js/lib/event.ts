import type { LogPayload } from './types';

type Severity = 'info' | 'warning' | 'error' | 'critical';

export class LogwolfEvent {
	private readonly start = performance.now();
	public readonly createdAt = new Date();

	constructor(
		public name: string,
		public severity: Severity,
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
		const now = performance.now();
		const duration = Math.floor(now - this.start);

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
