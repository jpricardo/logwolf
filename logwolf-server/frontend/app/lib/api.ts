type ApiResponse<T> = { message: string } & ({ error: true; data: never } | { error: false; data: T });

type ApiKey = {
	id: string;
	project_id: string;
	prefix: string;
	active: boolean;
	created_at: string;
	revoked_at: string;
};

export interface IApi {
	getKeys(): Promise<ApiKey[]>;
	createKey(projectId: string): Promise<{ key: string; prefix: string; id: string }>;
	deleteKey(id: string): Promise<void>;
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
}
