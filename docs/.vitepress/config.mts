import { defineConfig } from 'vitepress';

export default defineConfig({
	title: 'Logwolf',
	description: 'Self-hosted event logging and observability.',

	head: [['link', { rel: 'icon', href: '/favicon.ico' }]],

	themeConfig: {
		nav: [
			{ text: 'Guide', link: '/getting-started' },
			{ text: 'SDK', link: '/sdk/js' },
			{ text: 'GitHub', link: 'https://github.com/jpricardo/logwolf' },
		],

		sidebar: [
			{
				text: 'Introduction',
				items: [
					{ text: 'Getting started', link: '/getting-started' },
					{ text: 'Self-hosting', link: '/self-hosting' },
					{ text: 'Architecture', link: '/architecture' },
				],
			},
			{
				text: 'SDK',
				items: [{ text: 'JavaScript', link: '/sdk/js' }],
			},
		],

		socialLinks: [{ icon: 'github', link: 'https://github.com/jpricardo/logwolf' }],

		footer: {
			message: 'Released under the Apache 2.0 License.',
			copyright: 'Copyright © 2026 jpricardo',
		},

		search: {
			provider: 'local',
		},
	},
});
