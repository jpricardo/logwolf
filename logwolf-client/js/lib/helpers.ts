import type { ApiResponse } from './types';

export async function handleResponse<T>(res: Response) {
	const json = res.json() as Promise<ApiResponse<T>>;

	return json.then((r) => {
		if (r.error) throw new Error(r.message);
		return r.data;
	});
}
