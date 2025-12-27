import { hello } from './index';

describe('hello', () => {
	test('world', () => {
		const res = hello();
		expect(res).toEqual('world');
	});
});
