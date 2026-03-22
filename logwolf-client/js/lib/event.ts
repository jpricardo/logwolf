import { CreateLogwolfEventDTOSchema, type LogwolfEventDTO, type Severity } from './schema';

export class LogwolfEvent {
	public readonly random = Math.random();
	public readonly start = performance.now();
	public readonly createdAt = new Date();

	public name: LogwolfEventDTO['name'];
	public severity: LogwolfEventDTO['severity'];
	public readonly tags: LogwolfEventDTO['tags'];
	public readonly data: NonNullable<LogwolfEventDTO['data']> = {};

	private _duration: number | null = null;

	constructor(props: LogwolfEventDTO) {
		this.name = props.name;
		this.severity = props.severity;
		this.tags = props.tags;
		if (props.data) {
			this.data = props.data;
		}
	}

	/**
	 * Stops the stopwatch. Duration is frozen from construction to this call.
	 * Called automatically by `capture()` and `create()` — you only need to
	 * call this manually if you want to stop the clock before those methods.
	 *
	 * Calling `stop()` more than once is a no-op; the first call wins.
	 */
	public stop(): void {
		if (this._duration === null) {
			this._duration = Math.floor(performance.now() - this.start);
		}
	}

	public setName(n: string) {
		this.name = n;
	}

	public setSeverity(s: Severity) {
		this.severity = s;
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
		const duration = this._duration ?? Math.floor(performance.now() - this.start);

		const encoded = CreateLogwolfEventDTOSchema.encode({
			name: this.name,
			severity: this.severity,
			tags: Array.from(new Set(this.tags)),
			data: this.data,
			duration: duration,
		});

		return JSON.stringify(encoded);
	}
}
