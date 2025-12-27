import typescript from '@rollup/plugin-typescript';
import dts from 'rollup-plugin-dts';

const config = [
	{
		input: 'dist/lib/index.js',
		output: {
			file: 'dist/logwolf-client.js',
			format: 'cjs',
			sourcemap: true,
		},
		external: ['axios', 'os', 'url'],
		plugins: [typescript()],
	},

	{
		input: 'dist/lib/index.d.ts',
		output: {
			file: 'dist/logwolf-client.d.ts',
			format: 'es',
		},
		plugins: [dts()],
	},
];

export default config;
