export type LogEvent = {
	id: string;
	name: string;
	severity: string;
	tags: string[];
	data: Record<string, unknown>;
	duration?: number;
	created_at: string;
	updated_at: string;
};

export type LogPayload = Pick<LogEvent, 'name' | 'severity' | 'tags' | 'data' | 'duration'>;

export type ApiResponse<T> = { message: string } & ({ error: true } | { error: false; data: T });
