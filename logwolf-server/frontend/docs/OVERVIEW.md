# Frontend — Overview

## Purpose

Server-side-rendered React dashboard for managing and viewing Logwolf data. Provides authenticated views for log events, API keys, system settings, and usage analytics.

## Tech stack

| Layer          | Technology                                                  |
| -------------- | ----------------------------------------------------------- |
| Framework      | React Router v7 (SSR)                                       |
| UI             | React 19, Tailwind CSS 4, Radix UI, Shadcn-style components |
| Charts         | Recharts                                                    |
| Auth           | GitHub OAuth 2.0 + iron-session (secure cookies)            |
| Build          | Vite + TypeScript                                           |
| Error tracking | `@logwolf/client-js` (the SDK, eating its own dog food)     |

## Source layout

```
app/
├── root.tsx              # App root layout, global error boundary, Logwolf middleware
├── entry.server.tsx      # SSR entry point, streaming HTML render
├── routes.ts             # Route definitions
├── context.ts            # React context for event tracking
├── app.css               # Global Tailwind CSS
├── components/
│   ├── nav/              # Header, sidebar, theme picker, page wrapper
│   └── ui/               # 25+ reusable UI primitives (button, table, dialog, etc.)
├── pages/
│   ├── layout.tsx        # Authenticated layout wrapper
│   ├── home/             # Public landing page
│   ├── auth/             # GitHub OAuth login flow
│   ├── dashboard/        # Metrics overview + charts
│   ├── events/           # Event list, detail view, create form
│   ├── keys/             # API key management
│   ├── projects/         # Project list, create, switch, per-project settings
│   └── settings/         # Redirect to the current project's settings
├── lib/
│   ├── api.ts            # Dashboard API client (calls Broker internal routes)
│   ├── logwolf.ts        # Logwolf SDK setup for client-side error tracking
│   ├── auth.server.ts    # Server-side GitHub OAuth logic
│   ├── session.server.ts # iron-session cookie helpers
│   ├── csrf.server.ts    # CSRF token generation + validation
│   ├── format.ts         # Formatting utilities (dates, numbers)
│   ├── parse.ts          # Parsing utilities
│   ├── slug.ts           # Project name -> URL-safe slug
│   └── utils.ts          # General utilities
├── hooks/
│   ├── use-csrf-token.ts # Fetch CSRF token for form submissions
│   ├── use-projects.ts   # Read the user's projects from the layout loader
│   └── use-mobile.ts     # Detect mobile viewport
└── store/
    └── theme-provider.tsx # Dark/light mode provider (next-themes)
```

## Routes

| Path                     | Auth      | Description                                 |
| ------------------------ | --------- | ------------------------------------------- |
| `/`                      | Public    | Landing page                                |
| `/auth`                  | Public    | GitHub OAuth login                          |
| `/dashboard`             | Protected | Metrics overview with charts                |
| `/events`                | Protected | Paginated event list                        |
| `/events/create`         | Protected | Create a new event                          |
| `/events/:id`            | Protected | Event detail view                           |
| `/keys`                  | Protected | API key management                          |
| `/settings`              | Protected | Redirects to the current project's settings |
| `/projects`              | Protected | Projects the user belongs to                |
| `/projects/new`          | Protected | Create a project                            |
| `/projects/switch`       | Protected | POST-only project switch                    |
| `/projects/:id/settings` | Protected | Rename, retention, members, delete          |

## Project selection

Every protected page except `/projects/new` is scoped to one project, held in the
session as `currentProjectID`. The layout loader owns that value: it re-points the
session when the stored project is gone or the user lost access to it, and it sends
a user with no projects at all to `/projects/new` — the one page that renders
without a current project.

Switching projects is a POST to `/projects/switch`, which re-checks membership
server-side before writing the session. The sidebar switcher and the `/projects`
list both go through it.

## Project settings

`/projects/:id/settings` holds everything scoped to one project: its name, the
retention window, the member list, and deletion. Both the loader and the action
resolve the `:id` against the caller's own project list, so a project the user
does not belong to sends them back to `/projects` instead of surfacing a 403.

Every section except retention is owner-only — the broker enforces that as well,
so the role checks in the route are there to keep a stale tab from producing a
bare "forbidden". Deleting a project clears `currentProjectID` when it was the
one in session and returns to `/projects`, where the layout takes over.

## Authentication

1. User initiates login via GitHub OAuth 2.0.
2. On callback, the server checks the GitHub user against `GITHUB_ALLOWED_USERS` or `GITHUB_ALLOWED_ORGS`.
3. A signed iron-session cookie is issued for subsequent requests.
4. All protected routes validate the session server-side before rendering.

CSRF tokens are required on all mutating form submissions.

## API communication

`lib/api.ts` exports an `Api` class that calls Broker's **internal routes** using the `X-Internal-Secret` header (sourced from `INTERNAL_API_SECRET`). The frontend never calls the public Broker routes — those are for SDK clients only.

## Error tracking

`lib/logwolf.ts` initialises the Logwolf JS SDK. `root.tsx` wires it into the React Router middleware so every navigation and unhandled error is captured automatically.

## Environment variables

| Variable               | Description                                      |
| ---------------------- | ------------------------------------------------ |
| `API_URL`              | Broker base URL (e.g. `http://broker/`)          |
| `INTERNAL_API_SECRET`  | Shared secret for internal Broker routes         |
| `GITHUB_CLIENT_ID`     | GitHub OAuth app client ID                       |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret                   |
| `GITHUB_ALLOWED_USERS` | Comma-separated list of allowed GitHub usernames |
| `GITHUB_ALLOWED_ORGS`  | Comma-separated list of allowed GitHub orgs      |
| `SESSION_SECRET`       | Secret for iron-session cookie signing           |

Copy `.env.example` to `.env` before running locally.

## Development commands

```bash
npm run dev       # Vite dev server (hot reload)
npm run build     # react-router build → build/
npm run typecheck # react-router typegen + tsc --noEmit
npm run lint      # oxlint
```

## Relationship to other services

| Service | Relationship                                                        |
| ------- | ------------------------------------------------------------------- |
| Broker  | Frontend calls Broker internal routes for data and admin operations |
| Caddy   | Reverse-proxies public traffic to the Frontend                      |
| GitHub  | OAuth provider for user authentication                              |
