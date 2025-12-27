import { Logwolf } from './client';
import { LogwolfEvent } from './event';

const mockFetch = vi.fn().mockReturnValue(new Promise((resolve) => resolve(0)));

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
		const ev = new LogwolfEvent('Test', 'test', [], {});

		client.logEvent(ev);

		expect(mockFetch).toHaveBeenCalled();
		expect(mockFetch).toHaveBeenCalledWith(new URL('/posts', testUrl), { method: 'POST', body: ev.toJson() });
	});
});
