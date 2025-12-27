export type LogPayload = {
	name: string;
	severity: string;
	tags: string[];
	data: Record<string, unknown>;
};
