import z from 'zod';

const SeveritySchema = z.enum(['info', 'warning', 'error', 'critical']);
export type Severity = z.infer<typeof SeveritySchema>;

export const EventSchema = z.object({
	id: z.string(),
	name: z.string(),
	severity: SeveritySchema,
	tags: z.array(z.string()),
	data: z.unknown(),
	duration: z.int().optional(),
	// TODO - date
	created_at: z.string(),
	// TODO - date
	updated_at: z.string(),
});
export type Event = z.infer<typeof EventSchema>;

export const CreateEventDTOSchema = EventSchema.pick({
	name: true,
	severity: true,
	tags: true,
	data: true,
	duration: true,
});
export type CreateEventDTO = z.infer<typeof CreateEventDTOSchema>;

export const DeleteEventDTOSchema = EventSchema.pick({ id: true });
export type DeleteEventDTO = z.infer<typeof DeleteEventDTOSchema>;

export type ApiResponse<T> = { message: string } & ({ error: true } | { error: false; data: T });

export class EventsApi {
	static readonly apiUrl = process.env.API_URL;

	private static handleResponse<T>(r: ApiResponse<T>): T {
		if (r.error) throw new Error(r.message);
		return r.data;
	}

	// TODO - Return created item id
	static async create(p: CreateEventDTO) {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, {
			method: 'POST',
			body: JSON.stringify(p),
		})
			.then<ApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}

	static async getAll() {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'GET' })
			.then<ApiResponse<Event[]>>((r) => r.json())
			.then((r) => this.handleResponse(r));

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

	static async delete(dto: DeleteEventDTO) {
		const url = new URL('/logs', this.apiUrl);
		const res = await fetch(url, { method: 'DELETE', body: JSON.stringify(dto) })
			.then<ApiResponse<void>>((r) => r.json())
			.then((r) => this.handleResponse(r));

		return res;
	}
}
