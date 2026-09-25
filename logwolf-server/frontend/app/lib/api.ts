import {
	LogwolfEventSchema,
	type CreateLogwolfEventDTOSchema,
	type LogwolfEventData,
	type Pagination,
} from '@logwolf/client-js';
import z from 'zod';

type ApiResponse<T> = { message: string } & ({ error: true; data: never } | { error: false; data: T });

/**
 * An event the way the broker takes it in: `data` already encoded as a JSON
 * string. `LogwolfEvent.toObject()` produces exactly this shape.
 */
export type EncodedEvent = z.input<typeof CreateLogwolfEventDTOSchema>;

/** What a key may do on the public /logs routes. */
export const API_KEY_SCOPES = ['ingest', 'read', 'delete'] as const;
export type ApiKeyScope = (typeof API_KEY_SCOPES)[number];

type ApiKey = {
	id: string;
	project_id: string;
	prefix: string;
	scopes: ApiKeyScope[];
	/** Created before scopes existed: it keeps the full access it always had. */
	legacy?: boolean;
	active: boolean;
	created_at: string;
	revoked_at: string;
};

export type CreatedApiKey = { key: string; prefix: string; id: string; scopes: ApiKeyScope[] };

export type Project = {
	id: string;
	name: string;
	slug: string;
	created_at: string;
};

export type ProjectRole = 'owner' | 'member';

/** A project together with the role the requesting user holds in it. */
export type UserProject = Project & { role: ProjectRole };

/** A row of the project_members collection, as returned by the broker. */
export type ProjectMember = {
	id: string;
	project_id: string;
	github_login: string;
	role: ProjectRole;
	created_at: string;
};

export type RetentionDays = 0 | 30 | 60 | 90 | 180 | 365;

export type Metrics = {
	total_events: number;
	total_errors: number;
	total_critical: number;
	avg_duration_ms: number;
	events_last_24h: number;
	errors_last_24h: number;
	top_tags: { tag: string; count: number }[] | null;
};

export interface IApi {
	getProjects(): Promise<UserProject[]>;
	createProject(name: string, slug: string): Promise<Project>;
	updateProject(id: string, name: string): Promise<Project>;
	deleteProject(id: string): Promise<void>;
	getMembers(projectId: string): Promise<ProjectMember[]>;
	addMember(projectId: string, login: string, role: ProjectRole): Promise<void>;
	updateMemberRole(projectId: string, login: string, role: ProjectRole): Promise<void>;
	removeMember(projectId: string, login: string): Promise<void>;
	getKeys(projectId: string): Promise<ApiKey[]>;
	createKey(projectId: string, scopes: ApiKeyScope[]): Promise<CreatedApiKey>;
	deleteKey(projectId: string, id: string): Promise<void>;
	getRetention(projectId: string): Promise<{ days: RetentionDays }>;
	updateRetention(projectId: string, days: number): Promise<{ days: RetentionDays }>;
	getMetrics(projectId: string): Promise<Metrics>;
	getLogs(projectId: string, p: Pagination): Promise<LogwolfEventData[]>;
	getLog(projectId: string, id: string): Promise<LogwolfEventData>;
	createLog(projectId: string, event: EncodedEvent): Promise<void>;
	deleteLog(projectId: string, id: string): Promise<void>;
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

	public async getProjects(): Promise<UserProject[]> {
		const res = await fetch(`${this.baseUrl}projects`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<UserProject[]>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async createProject(name: string, slug: string): Promise<Project> {
		const res = await fetch(`${this.baseUrl}projects`, {
			method: 'POST',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ name, slug }),
		});
		const json = (await res.json()) as ApiResponse<Project>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async updateProject(id: string, name: string): Promise<Project> {
		const res = await fetch(`${this.baseUrl}projects/${id}`, {
			method: 'PATCH',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ name }),
		});
		const json = (await res.json()) as ApiResponse<Project>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async deleteProject(id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}projects/${id}`, {
			method: 'DELETE',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}

	public async getMembers(projectId: string): Promise<ProjectMember[]> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/members`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<ProjectMember[]>;
		if (json.error) throw new Error(json.message);

		return json.data ?? [];
	}

	public async addMember(projectId: string, login: string, role: ProjectRole): Promise<void> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/members`, {
			method: 'POST',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ login, role }),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}

	public async updateMemberRole(projectId: string, login: string, role: ProjectRole): Promise<void> {
		// Encoded for the same reason as in removeMember.
		const res = await fetch(`${this.baseUrl}projects/${projectId}/members/${encodeURIComponent(login)}`, {
			method: 'PATCH',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ role }),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}

	public async removeMember(projectId: string, login: string): Promise<void> {
		// GitHub logins are URL-safe, but the value reaches us from a form field —
		// encoding it keeps a hand-crafted login from reshaping the path.
		const res = await fetch(`${this.baseUrl}projects/${projectId}/members/${encodeURIComponent(login)}`, {
			method: 'DELETE',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}

	public async getKeys(projectId: string): Promise<ApiKey[]> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/keys`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<ApiKey[]>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	/** An empty `scopes` leaves the choice to the broker, which gives the key ingest only. */
	public async createKey(projectId: string, scopes: ApiKeyScope[]): Promise<CreatedApiKey> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/keys`, {
			method: 'POST',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ scopes }),
		});
		const json = (await res.json()) as ApiResponse<CreatedApiKey>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	/** Revokes a key of the project; another project's key is "key not found". */
	public async deleteKey(projectId: string, id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/keys/${encodeURIComponent(id)}`, {
			method: 'DELETE',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
		return json.data;
	}

	public async getRetention(projectId: string): Promise<{ days: RetentionDays }> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/retention`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async updateRetention(projectId: string, days: number): Promise<{ days: RetentionDays }> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/retention`, {
			method: 'PATCH',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ days }),
		});
		const json = (await res.json()) as ApiResponse<{ days: RetentionDays }>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async getMetrics(projectId: string): Promise<Metrics> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/metrics`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<Metrics>;
		if (json.error) throw new Error(json.message);

		return json.data;
	}

	public async getLogs(projectId: string, p: Pagination): Promise<LogwolfEventData[]> {
		const url = new URL(`${this.baseUrl}projects/${projectId}/logs`);
		url.searchParams.set('page', String(p.page));
		url.searchParams.set('pageSize', String(p.pageSize));
		const res = await fetch(url.toString(), {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<unknown[]>;
		if (json.error) throw new Error(json.message);

		return LogwolfEventSchema.array().parse(json.data ?? []);
	}

	public async getLog(projectId: string, id: string): Promise<LogwolfEventData> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/logs/${encodeURIComponent(id)}`, {
			method: 'GET',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<unknown>;
		if (json.error) throw new Error(json.message);

		return LogwolfEventSchema.parse(json.data);
	}

	public async createLog(projectId: string, event: EncodedEvent): Promise<void> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/logs`, {
			method: 'POST',
			headers: this.internalHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(event),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}

	public async deleteLog(projectId: string, id: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}projects/${projectId}/logs/${encodeURIComponent(id)}`, {
			method: 'DELETE',
			headers: this.internalHeaders(),
		});
		const json = (await res.json()) as ApiResponse<void>;
		if (json.error) throw new Error(json.message);
	}
}

/**
 * Create a request-scoped API client.
 * userLogin must come from the authenticated GitHub session.
 */
export function createApi(userLogin: string): IApi {
	return new Api(process.env.API_URL!, process.env.INTERNAL_API_SECRET!, userLogin);
}
