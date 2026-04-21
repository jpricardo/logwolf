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
