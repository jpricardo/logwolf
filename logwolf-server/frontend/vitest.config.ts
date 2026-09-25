import { defineConfig } from 'vitest/config';

// Kept apart from vite.config.ts: the React Router plugin expects a full app
// build, and the tests only exercise server-side modules: libraries, and route
// loaders and actions, called the way React Router calls them.
export default defineConfig({
	resolve: { tsconfigPaths: true },
	test: {
		environment: 'node',
		include: ['app/**/*.test.ts'],
		// Every test starts with fresh call records on the mocks it shares.
		clearMocks: true,
		// Route tests sign real session cookies; session.server reads the secret
		// when it is first imported.
		env: { SESSION_SECRET: 'test-session-secret-0123456789abcdef' },
	},
});
