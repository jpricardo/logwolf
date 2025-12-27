export type Log = {
	id: string;
	name: string;
	severity: string;
	tags: string[];
	data: { duration?: number } & Record<string, unknown>;
	created_at: string;
	updated_at: string;
};

export type ApiResponse<T> = { message: string } & ({ error: true } | { error: false; data: T });

export class Api {
	static readonly apiUrl = process.env.API_URL;

	static async getLogs() {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'GET' })
			.then((r) => r.json() as Promise<ApiResponse<Log[]>>)
			.then((r) => {
				if (r.error) {
					throw new Error(r.message);
				}

				return r.data;
			});

		return res;
	}
}
