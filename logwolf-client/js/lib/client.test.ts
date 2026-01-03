import { Logwolf } from './client';
import { LogwolfEvent } from './event';

const mockFetch = vi.fn().mockReturnValue(
	new Promise((resolve) => {
		return resolve({
			json: vi.fn().mockResolvedValue({ error: false, data: [] }),
		});
	}),
);

vi.stubGlobal('fetch', mockFetch);

describe('Logwolf', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('should log events correctly', () => {
		const testUrl = 'http://test.url';
		const client = new Logwolf(testUrl);
		const ev = new LogwolfEvent({ name: 'Test', severity: 'info', tags: [], data: {} });

		client.create(ev);

		expect(mockFetch).toHaveBeenCalled();
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testUrl), { method: 'POST', body: ev.toJson() });
	});

	it('should get events correctly', () => {
		const testUrl = 'http://test.url';
		const client = new Logwolf(testUrl);

		client.getAll();

		expect(mockFetch).toHaveBeenCalled();
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testUrl), { method: 'GET' });
	});

	it('should delete events correctly', () => {
		const testUrl = 'http://test.url';
		const client = new Logwolf(testUrl);

		client.delete({ id: 'id' });

		expect(mockFetch).toHaveBeenCalled();
		expect(mockFetch).toHaveBeenCalledWith(new URL('/logs', testUrl), {
			method: 'DELETE',
			body: JSON.stringify({ id: 'id' }),
		});
	});
});
