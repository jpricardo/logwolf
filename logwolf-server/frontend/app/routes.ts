import { type RouteConfig, index, layout, route } from '@react-router/dev/routes';

export default [
	index('pages/home/index.tsx'),
	route('auth', 'pages/auth/index.tsx'),

	layout('pages/layout.tsx', [
		route('dashboard', 'pages/dashboard/index.tsx'),
		route('events', 'pages/events/index.tsx'),
		route('events/:id', 'pages/events/details/index.tsx'),
		route('events/new', 'pages/events/create/index.tsx'),
		route('keys', 'pages/keys/index.tsx'),
		route('settings', 'pages/settings/index.tsx'),
		route('projects', 'pages/projects/index.tsx'),
		route('projects/new', 'pages/projects/create/index.tsx'),
		route('projects/switch', 'pages/projects/switch/index.tsx'),
	]),
] satisfies RouteConfig;
