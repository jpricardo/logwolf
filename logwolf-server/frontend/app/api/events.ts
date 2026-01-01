export type Event = {
	id: string;
	name: string;
	severity: string;
	tags: string[];
	data: { duration?: number } & Record<string, unknown>;
	created_at: string;
	updated_at: string;
};

export type ApiResponse<T> = { message: string } & ({ error: true } | { error: false; data: T });

export class EventsApi {
	static readonly apiUrl = process.env.API_URL;

	static async getAll() {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'GET' })
			.then<ApiResponse<Event[]>>((r) => r.json())
			.then((r) => {
				if (r.error) {
					throw new Error(r.message);
				}

				return r.data.toSorted((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
			});

		return res;
	}

	static async getOne(id: string) {
		const res = this.getAll().then((r) => {
			return r.find((i) => i.id === id);
		});

		return res;
	}

	static async getRelated(id: string, amt: number) {
		// TODO - "relatedness" algorithm
		const res = this.getAll().then((r) => r.filter((i) => i.id !== id).slice(0, amt));
		return res;
	}
}
