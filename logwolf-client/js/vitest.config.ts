import { defineConfig } from 'vitest/config';

export default defineConfig({
	base: '/',

	test: {
		silent: 'passed-only',
		globals: true,
		environment: 'node',
		sequence: {
			setupFiles: 'list',
			hooks: 'list',
		},

		coverage: {
			provider: 'v8',
			reporter: ['json', 'lcov', 'html'],
		},
	},
});
