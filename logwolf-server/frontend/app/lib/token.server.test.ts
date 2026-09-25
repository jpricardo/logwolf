import { afterEach, describe, expect, it, vi } from 'vitest';

import { sealToken, unsealToken } from './token.server';

describe('sealToken', () => {
	afterEach(() => vi.unstubAllEnvs());

	it('round-trips, and does not carry the token in the clear', () => {
		const sealed = sealToken('gho_secret123');

		expect(unsealToken(sealed)).toBe('gho_secret123');
		expect(sealed).not.toContain('gho_secret123');
		expect(Buffer.from(sealed, 'base64url').toString('utf8')).not.toContain('gho_secret123');
	});

	it('seals the same token differently each time', () => {
		expect(sealToken('gho_secret123')).not.toBe(sealToken('gho_secret123'));
	});

	it('refuses a value that was altered, or is not one at all', () => {
		const sealed = Buffer.from(sealToken('gho_secret123'), 'base64url');
		sealed[sealed.length - 1]! ^= 1;

		expect(unsealToken(sealed.toString('base64url'))).toBeUndefined();
		expect(unsealToken('not-a-sealed-token')).toBeUndefined();
		expect(unsealToken('')).toBeUndefined();
		expect(unsealToken()).toBeUndefined();
	});

	it('refuses a token sealed under another SESSION_SECRET', () => {
		const sealed = sealToken('gho_secret123');
		vi.stubEnv('SESSION_SECRET', 'a-different-secret');

		expect(unsealToken(sealed)).toBeUndefined();
	});
});
