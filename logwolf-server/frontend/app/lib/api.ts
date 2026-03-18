type ApiResponse<T> = { message: string } & ({ error: true; data: never } | { error: false; data: T });

type ApiKey = {
	id: string;
	project_id: string;
	prefix: string;
	active: boolean;
	created_at: string;
	revoked_at: string;
};

export type RetentionDays = 0 | 30 | 60 | 90 | 180 | 365;

export type Metrics = {
	total_events: number;
	total_errors: number;
	total_critical: number;
	avg_duration_ms: number;
	events_last_24h: number;
	errors_last_24h: number;
	top_tags: { tag: string; count: number }[];
};

export interface IApi {
	getKeys(): Promise<ApiKey[]>;
	createKey(projectId: string): Promise<{ key: string; prefix: string; id: string }>;
	deleteKey(id: string): Promise<void>;
	getRetention(): Promise<{ days: RetentionDays }>;
	updateRetention(days: number): Promise<{ days: RetentionDays }>;
	getMetrics(): Promise<Metrics>;
}

export class Api implements IApi {
	constructor(
		private readonly baseUrl: string,
		private readonly secret: string,
	) {}

	public async getKeys(): Promise<ApiKey[]> {
		const res = await fetch(`${this.baseUrl}keys`, {
			method: 'GET',
			headers: { 'X-Internal-Secret': this.secret },
		});
		const json = (await res.json()) as ApiResponse<ApiKey[]>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async createKey(projectId: string): Promise<{ key: string; prefix: string; id: string }> {
		const res = await fetch(`${this.baseUrl}keys`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', 'X-Internal-Secret': this.secret },
			body: JSON.stringify({ project_id: projectId }),
		});
		const json = (await res.json()) as ApiResponse<{ key: string; prefix: string; id: string }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async deleteKey(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}keys/${id}`, {
			method: 'DELETE',
			headers: { 'X-Internal-Secret': this.secret },
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
		return json.data;
	}

	public async getRetention(): Promise<{ days: RetentionDays }> {
		const res = await fetch(`${this.baseUrl}settings/retention`, {
			method: 'GET',
			headers: { 'X-Internal-Secret': this.secret },
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async updateRetention(days: number): Promise<{ days: RetentionDays }> {
		const res = await fetch(`${this.baseUrl}settings/retention`, {
			method: 'PATCH',
			headers: { 'X-Internal-Secret': this.secret },
			body: JSON.stringify({ days }),
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async getMetrics(): Promise<Metrics> {
		const res = await fetch(`${this.baseUrl}metrics`, {
			method: 'GET',
			headers: { 'X-Internal-Secret': this.secret },
		});
		const json = (await res.json()) as ApiResponse<Metrics>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}
}

export const api = new Api(process.env.API_URL!, process.env.INTERNAL_API_SECRET!);
