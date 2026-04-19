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
	getKeys(projectId: string): Promise<ApiKey[]>;
	createKey(projectId: string): Promise<{ key: string; prefix: string; id: string }>;
	deleteKey(id: string): Promise<void>;
	getRetention(projectId: string): Promise<{ days: RetentionDays }>;
	updateRetention(projectId: string, days: number): Promise<{ days: RetentionDays }>;
	getMetrics(projectId: string): Promise<Metrics>;
}

export class Api implements IApi {
	constructor(
		private readonly baseUrl: string,
		private readonly secret: string,
		private readonly userLogin: string,
	) {}

	private internalHeaders(extra?: Record<string, string>): Record<string, string> {
		return {
			'X-Internal-Secret': this.secret,
			'X-User-Login': this.userLogin,
			...extra,
		};
	}

	public async getKeys(projectId: string): Promise<ApiKey[]> {
		const url = new URL(`${this.baseUrl}keys`);
		url.searchParams.set('project_id', projectId);
		const res = await fetch(url.toString(), {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<ApiKey[]>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async createKey(projectId: string): Promise<{ key: string; prefix: string; id: string }> {
		const res = await fetch(`${this.baseUrl}keys`, {
			method: 'POST',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ project_id: projectId }),
		});
		const json = (await res.json()) as ApiResponse<{ key: string; prefix: string; id: string }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async deleteKey(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}keys/${id}`, {
			method: 'DELETE',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
		return json.data;
	}

	public async getRetention(projectId: string): Promise<{ days: RetentionDays }> {
		const url = new URL(`${this.baseUrl}settings/retention`);
		url.searchParams.set('project_id', projectId);
		const res = await fetch(url.toString(), {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async updateRetention(projectId: string, days: number): Promise<{ days: RetentionDays }> {
		const res = await fetch(`${this.baseUrl}settings/retention`, {
			method: 'PATCH',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ project_id: projectId, days }),
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async getMetrics(projectId: string): Promise<Metrics> {
		const url = new URL(`${this.baseUrl}metrics`);
		url.searchParams.set('project_id', projectId);
		const res = await fetch(url.toString(), {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<Metrics>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}
}

/**
 * Create a request-scoped API client.
 * userLogin must come from the authenticated GitHub session.
 */
export function createApi(userLogin: string): IApi {
	return new Api(process.env.API_URL!, process.env.INTERNAL_API_SECRET!, userLogin);
}
