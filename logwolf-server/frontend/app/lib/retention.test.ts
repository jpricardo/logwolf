import { describe, expect, it } from 'vitest';

import { lowersRetention } from './retention';

describe('lowersRetention', () => {
	it('is true for a shorter window', () => {
		expect(lowersRetention(90, 30)).toBe(true);
		expect(lowersRetention(365, 180)).toBe(true);
	});

	it('treats forever as the longest window', () => {
		expect(lowersRetention(0, 365)).toBe(true);
		expect(lowersRetention(365, 0)).toBe(false);
		expect(lowersRetention(0, 0)).toBe(false);
	});

	it('is false for a longer or unchanged window', () => {
		expect(lowersRetention(30, 90)).toBe(false);
		expect(lowersRetention(90, 90)).toBe(false);
	});
});
