import { defineConfig } from 'vitest/config';

// Kept apart from vite.config.ts: the React Router plugin expects a full app
// build, and the tests only exercise server-side modules.
export default defineConfig({
	resolve: { tsconfigPaths: true },
	test: {
		environment: 'node',
		include: ['app/**/*.test.ts'],
	},
});
