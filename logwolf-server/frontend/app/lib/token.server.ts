import { createCipheriv, createDecipheriv, createHash, randomBytes } from 'node:crypto';

// The session cookie is signed, not encrypted: anyone holding it can read what
// is in it. The user's GitHub token goes in sealed, with AES-256-GCM under a key
// derived from SESSION_SECRET, so the cookie carries it without showing it.

function key(): Buffer {
	return createHash('sha256')
		.update('logwolf:github-token:')
		.update(process.env.SESSION_SECRET ?? '')
		.digest();
}

/** Encrypts a GitHub token for the session cookie. */
export function sealToken(token: string): string {
	const iv = randomBytes(12);
	const cipher = createCipheriv('aes-256-gcm', key(), iv);
	const sealed = Buffer.concat([cipher.update(token, 'utf8'), cipher.final()]);
	return Buffer.concat([iv, cipher.getAuthTag(), sealed]).toString('base64url');
}

/**
 * Decrypts what sealToken made, or undefined for anything else: nothing, a
 * value tampered with, or one sealed under another SESSION_SECRET.
 */
export function unsealToken(sealed?: string): string | undefined {
	if (!sealed) return undefined;
	try {
		const raw = Buffer.from(sealed, 'base64url');
		const decipher = createDecipheriv('aes-256-gcm', key(), raw.subarray(0, 12));
		decipher.setAuthTag(raw.subarray(12, 28));
		return Buffer.concat([decipher.update(raw.subarray(28)), decipher.final()]).toString('utf8');
	} catch {
		return undefined;
	}
}
