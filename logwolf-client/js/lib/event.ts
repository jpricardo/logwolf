import { CreateLogwolfEventDTOSchema, type LogwolfEventDTO } from './schema';

export class LogwolfEvent {
	public readonly random = Math.random();
	public readonly start = performance.now();
	public readonly createdAt = new Date();

	public name: LogwolfEventDTO['name'];
	public severity: LogwolfEventDTO['severity'];
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

	public addTag(t: string) {
		this.tags.push(t);
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
