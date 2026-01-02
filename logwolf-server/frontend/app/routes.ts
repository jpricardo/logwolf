import { type RouteConfig, index, layout, route } from '@react-router/dev/routes';

export default [
	index('pages/home/index.tsx'),
	route('auth', 'pages/auth/index.tsx'),

	layout('pages/layout.tsx', [
		route('dashboard', 'pages/dashboard/index.tsx'),
		route('events', 'pages/events/index.tsx'),
		route('events/:id', 'pages/events/details/index.tsx'),
		route('events/new', 'pages/events/create/index.tsx'),
	]),
] satisfies RouteConfig;
