import { createCookieSessionStorage } from 'react-router';

type SessionData = {
	githubUser: {
		login: string;
		name: string;
		avatarUrl: string;
	};
	csrfToken: string;
	currentProjectID: string;
};

export const sessionStorage = createCookieSessionStorage<SessionData>({
	cookie: {
		name: '__logwolf_session',
		httpOnly: true,
		secure: process.env.NODE_ENV === 'production',
		sameSite: 'strict',
		secrets: [process.env.SESSION_SECRET!],
		maxAge: 60 * 60 * 24 * 7, // 1 week
	},
});

export const { getSession, commitSession, destroySession } = sessionStorage;

/**
 * Reads the project the user is currently working in. Undefined means the user
 * has no reachable project — the layout loader clears the id whenever the
 * stored project is gone or the user lost access to it.
 */
export async function getCurrentProjectID(request: Request): Promise<string | undefined> {
	const session = await getSession(request.headers.get('Cookie'));
	return session.get('currentProjectID');
}
