import { LogwolfEvent } from './event';

describe('LogwolfEvent', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('should be initialized correctly', () => {
		const ev = new LogwolfEvent({ name: 'Test', severity: 'info', tags: [], data: {} });

		expect(ev.name).toEqual('Test');
		expect(ev.severity).toEqual('info');
		expect(ev.tags).toEqual([]);
		expect(ev.data).toEqual({});
	});

	it('should update data correctly', () => {
		const ev = new LogwolfEvent({ name: 'Test', severity: 'info', tags: [], data: {} });

		ev.set('testKey', 'testValue');

		expect(ev.data['testKey']).toBeDefined();
		expect(ev.get('testKey')).toEqual('testValue');
	});

	it('should inject `duration` while serializing', () => {
		const ev = new LogwolfEvent({ name: 'Test', severity: 'info', tags: [], data: {} });

		vi.advanceTimersByTime(200);
		const now = new Date();
		const elapsed = now.getTime() - ev.createdAt.getTime();

		expect(ev.toJson()).toMatch(`"duration":${elapsed}`);
	});
});
