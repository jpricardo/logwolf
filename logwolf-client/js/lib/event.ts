import type z from 'zod';

import { CreateLogwolfEventDTOSchema, LogwolfEventSchema } from './schema';

export const LogwolfEventDTOSchema = LogwolfEventSchema.pick({
	name: true,
	severity: true,
	tags: true,
	data: true,
}).partial({
	data: true,
});
export type LogwolfEventDTO = z.infer<typeof LogwolfEventDTOSchema>;

export class LogwolfEvent {
	private readonly start = performance.now();
	public readonly createdAt = new Date();

	public readonly name: LogwolfEventDTO['name'];
	public readonly severity: LogwolfEventDTO['severity'];
	public readonly tags: LogwolfEventDTO['tags'];
	public readonly data: NonNullable<LogwolfEventDTO['data']> = {};

	constructor(props: LogwolfEventDTO) {
		this.name = props.name;
		this.severity = props.severity;
		this.tags = props.tags;
		if (props.data) {
			this.data = props.data;
		}
	}

	public set(key: string, value: unknown) {
		this.data[key] = value;
	}

	public get(key: string) {
		return this.data[key];
	}

	public toJson() {
		const now = performance.now();
		const duration = Math.floor(now - this.start);

		const encoded = CreateLogwolfEventDTOSchema.encode({
			name: this.name,
			severity: this.severity,
			tags: this.tags,
			data: this.data,
			duration: duration,
		});

		return JSON.stringify(encoded);
	}
}
