import { data } from 'react-router';

import { getSession } from './session.server';

/**
 * Returns the CSRF token for the current session, generating one if absent.
 * Works against a single shared session instance so committing it won't drop
 * other session fields (e.g. githubUser) that were set in the same request.
 *
 * Usage in a loader:
 *
 *   const session = await getSession(request.headers.get('Cookie'));
 *   const csrfToken = getOrCreateCsrfToken(session);
 *   return data({ csrfToken }, {
 *     headers: { 'Set-Cookie': await commitSession(session) },
 *   });
 */
export function getOrCreateCsrfToken(session: Awaited<ReturnType<typeof getSession>>): string {
	let token = session.get('csrfToken');
	if (!token) {
		token = crypto.randomUUID();
		session.set('csrfToken', token);
	}
	return token;
}

/**
 * Validates the CSRF token submitted with a form action.
 * Throws a 403 response if the token is missing or doesn't match the session.
 * Call this at the top of every action that mutates state.
 */
export async function validateCsrfToken(request: Request, formData: FormData): Promise<void> {
	const session = await getSession(request.headers.get('Cookie'));
	const sessionToken = session.get('csrfToken');
	const formToken = formData.get('_csrf')?.toString();

	if (!sessionToken || !formToken || sessionToken !== formToken) {
		throw data({ error: true, message: 'Invalid CSRF token' }, { status: 403 });
	}
}
