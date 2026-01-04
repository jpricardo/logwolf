import z from 'zod';

export const LogwolfEventSeveritySchema = z.enum(['info', 'warning', 'error', 'critical']);
export type Severity = z.infer<typeof LogwolfEventSeveritySchema>;

export const LogwolfEventDataSchema = z.codec(z.string(), z.record(z.string(), z.any()), {
	decode: (jsonString, ctx) => {
		try {
			return JSON.parse(jsonString);
		} catch (err: any) {
			ctx.issues.push({
				code: 'invalid_format',
				format: 'json',
				input: jsonString,
				message: err.message,
			});
			return z.NEVER;
		}
	},
	encode: (value) => JSON.stringify(value),
});

export const LogwolfDatetimeSchema = z.codec(z.iso.datetime(), z.date(), {
	decode: (isoString) => new Date(isoString),
	encode: (date) => date.toISOString(),
});

export const LogwolfEventSchema = z.object({
	id: z.string(),
	name: z.string(),
	severity: LogwolfEventSeveritySchema,
	tags: z.array(z.string()),
	data: LogwolfEventDataSchema,
	duration: z.int().optional(),
	created_at: LogwolfDatetimeSchema,
	updated_at: LogwolfDatetimeSchema,
});

export type LogwolfEventData = z.infer<typeof LogwolfEventSchema>;

export const CreateLogwolfEventDTOSchema = LogwolfEventSchema.pick({
	name: true,
	severity: true,
	tags: true,
	data: true,
	duration: true,
});
export type CreateLogwolfEventDTO = z.infer<typeof CreateLogwolfEventDTOSchema>;

export const DeleteLogwolfEventDTOSchema = LogwolfEventSchema.pick({ id: true });
export type DeleteLogwolfEventDTO = z.infer<typeof DeleteLogwolfEventDTOSchema>;

export type LogwolfApiResponse<T> = { message: string } & ({ error: true } | { error: false; data: T });

export const LogwolfConfigSchema = z.object({
	url: z.url(),
	sampleRate: z.number().positive().lte(1).optional(),
	errorSampleRate: z.number().positive().lte(1).optional(),
});

export type LogwolfConfig = z.infer<typeof LogwolfConfigSchema>;

export const LogwolfEventDTOSchema = LogwolfEventSchema.pick({
	name: true,
	severity: true,
	tags: true,
	data: true,
}).partial({
	data: true,
});
export type LogwolfEventDTO = z.infer<typeof LogwolfEventDTOSchema>;

export const PaginationSchema = z.codec(
	z.instanceof(URLSearchParams),
	z.object({
		page: z.number().positive(),
		pageSize: z.number().positive(),
	}),
	{
		encode: (v) => {
			return new URLSearchParams({ page: '' + v.page, pageSize: '' + v.pageSize });
		},
		decode: (v) => {
			const page = v.get('page') ?? '0';
			const pageSize = v.get('pageSize') ?? '0';

			return { page: parseInt(page), pageSize: parseInt(pageSize) };
		},
	},
);

export type Pagination = z.infer<typeof PaginationSchema>;
