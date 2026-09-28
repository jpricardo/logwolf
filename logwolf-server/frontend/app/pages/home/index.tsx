import {
	Activity,
	ArrowRight,
	Clock,
	Database,
	FolderKanban,
	KeyRound,
	Layers,
	RefreshCw,
	ShieldCheck,
	type LucideIcon,
} from 'lucide-react';
import { Link } from 'react-router';

import { Logo, LogoMark } from '~/components/nav/logo';
import { ThemePicker } from '~/components/nav/theme-picker';
import { Button } from '~/components/ui/button';
import { cn } from '~/lib/utils';
import { ThemeProvider } from '~/store/theme-provider';

import { CodeBlock } from './components/code-block';
import { EventFeed } from './components/event-feed';
import { Pipeline } from './components/pipeline';

const GITHUB_URL = 'https://github.com/jpricardo/logwolf';
const DOCS_URL = 'https://logwolf-docs.vercel.app';
const NPM_URL = 'https://www.npmjs.com/package/@logwolf/client-js';

export function meta() {
	return [
		{ title: 'Logwolf — self-hosted event logging' },
		{
			name: 'description',
			content:
				'Self-hosted event logging and observability. Your logs stay on your server. No vendor lock-in, no per-seat pricing.',
		},
	];
}

const features: { icon: LucideIcon; title: string; body: string }[] = [
	{
		icon: Layers,
		title: 'Structured events',
		body: 'Severity, tags and arbitrary key/value payloads on every event.',
	},
	{
		icon: Activity,
		title: 'Off the hot path',
		body: 'capture() returns at once. The SDK buffers events and flushes them in configurable batches.',
	},
	{
		icon: RefreshCw,
		title: 'Retry with back-off',
		body: 'Failed sends are retried automatically. Auth errors are not, so a bad key fails loudly.',
	},
	{
		icon: Clock,
		title: 'Durations built in',
		body: 'Every LogwolfEvent is a stopwatch, frozen when it is enqueued. No manual timing code.',
	},
	{
		icon: ShieldCheck,
		title: 'At-least-once delivery',
		body: 'A 202 means RabbitMQ has confirmed the event. A logger outage delays events, never drops them.',
	},
	{
		icon: FolderKanban,
		title: 'Projects and members',
		body: 'Keep each app in its own project. Sign in with GitHub, invite teammates as members or owners.',
	},
	{
		icon: KeyRound,
		title: 'Scoped API keys',
		body: 'Keys carry ingest, read or delete scopes. New keys can only ingest, so they are safe in a browser bundle.',
	},
	{
		icon: Database,
		title: 'Retention per project',
		body: 'Keep logs for 30 days, a year, or forever. Expired events are cleaned up project by project.',
	},
];

const serverCode = `git clone https://github.com/jpricardo/logwolf.git
cd logwolf/logwolf-server
cp .env.example .env   # GitHub OAuth credentials and secrets
docker compose up --build -d`;

const sdkCode = `import Logwolf, { LogwolfEvent } from '@logwolf/client-js';

const logwolf = new Logwolf({
  url: 'https://logs.example.com/api/',
  apiKey: process.env.LOGWOLF_API_KEY,
});

const event = new LogwolfEvent({
  name: 'checkout.completed',
  severity: 'info',
  tags: ['payments'],
});

event.set('amount', 9900);
logwolf.capture(event); // enqueues and returns immediately`;

function SiteHeader() {
	return (
		<header className='sticky top-0 z-30 border-b bg-background/80 backdrop-blur-md'>
			<div className='mx-auto flex h-14 max-w-6xl items-center justify-between px-6'>
				<Link to='/'>
					<Logo />
				</Link>

				<nav className='flex items-center gap-1'>
					<Button asChild variant='ghost' size='sm' className='hidden sm:inline-flex'>
						<a href={DOCS_URL}>Docs</a>
					</Button>
					<Button asChild variant='ghost' size='sm' className='hidden sm:inline-flex'>
						<a href={GITHUB_URL}>GitHub</a>
					</Button>
					<ThemePicker />
					<Button asChild size='sm' className='ml-1'>
						<Link to='/dashboard'>Sign in</Link>
					</Button>
				</nav>
			</div>
		</header>
	);
}

function StepNumber({ n }: { n: number }) {
	return (
		<span className='flex size-5 items-center justify-center rounded-sm bg-primary font-mono text-[11px] font-semibold text-primary-foreground'>
			{n}
		</span>
	);
}

function Section({ className, children, ...props }: React.ComponentProps<'section'>) {
	return (
		<section className={cn('border-b', className)} {...props}>
			<div className='mx-auto max-w-6xl px-6 py-20'>{children}</div>
		</section>
	);
}

function SectionHeading({ eyebrow, title, children }: { eyebrow: string; title: string; children?: React.ReactNode }) {
	return (
		<div className='mb-10 max-w-2xl'>
			<p className='mb-3 font-mono text-xs tracking-wide text-primary uppercase'>{eyebrow}</p>
			<h2 className='text-3xl font-semibold tracking-tight text-balance sm:text-4xl'>{title}</h2>
			{children && <p className='mt-3 text-muted-foreground'>{children}</p>}
		</div>
	);
}

export default function Home() {
	return (
		<ThemeProvider>
			<div className='min-h-screen'>
				<SiteHeader />

				<main>
					<Section className='relative overflow-hidden'>
						<div className='bg-grid pointer-events-none absolute inset-0' aria-hidden />
						<div
							className='pointer-events-none absolute -top-40 left-1/2 h-80 w-[48rem] -translate-x-1/2 bg-[radial-gradient(closest-side,var(--color-primary),transparent)] opacity-15 dark:opacity-20'
							aria-hidden
						/>

						<div className='relative grid grid-cols-1 items-center gap-12 lg:grid-cols-[1fr_1.1fr]'>
							<div>
								<p className='mb-6 inline-flex items-center gap-2 rounded-sm border bg-card px-2.5 py-1 font-mono text-xs tracking-wide text-muted-foreground uppercase'>
									<span className='size-1.5 rounded-[1px] bg-primary' aria-hidden />
									Self-hosted · Open source · GPL v3
								</p>
								<h1 className='text-4xl font-semibold tracking-tight text-balance sm:text-6xl'>
									Your logs stay on <br className='hidden sm:block' />
									<span className='text-primary'>your server.</span>
								</h1>
								<p className='mt-5 max-w-xl text-lg text-muted-foreground'>
									Logwolf is event logging for developers who want to own their data. No vendor lock-in, no per-seat
									pricing. One <code className='font-mono text-foreground'>docker compose up</code> and you're logging.
								</p>

								<div className='mt-8 flex flex-wrap gap-3'>
									<Button asChild size='lg'>
										<a href={`${DOCS_URL}/getting-started.html`}>Get started</a>
									</Button>
									<Button asChild size='lg' variant='outline'>
										<a href={GITHUB_URL}>View on GitHub</a>
									</Button>
								</div>

								<p className='mt-8 inline-flex items-center gap-2 rounded-md border bg-card px-3 py-2 font-mono text-[13px] text-muted-foreground'>
									<span className='text-primary select-none'>$</span>
									<span className='text-foreground'>npm install @logwolf/client-js</span>
								</p>
							</div>

							<EventFeed className='shadow-xl shadow-black/5 dark:shadow-black/40' />
						</div>
					</Section>

					<Section id='features'>
						<SectionHeading eyebrow='Features' title='Everything you need, nothing you rent'>
							A small SDK, a Go pipeline and a dashboard. Run the whole thing next to your app.
						</SectionHeading>

						<div className='grid grid-cols-1 gap-px overflow-hidden rounded-lg border bg-border sm:grid-cols-2 lg:grid-cols-4'>
							{features.map((f) => (
								<div key={f.title} className='bg-card p-6 transition-colors hover:bg-accent/40'>
									<span className='mb-4 flex size-9 items-center justify-center rounded-md border border-primary/20 bg-primary/10'>
										<f.icon className='size-4.5 text-primary' />
									</span>
									<h3 className='font-medium'>{f.title}</h3>
									<p className='mt-2 text-sm text-muted-foreground'>{f.body}</p>
								</div>
							))}
						</div>
					</Section>

					<Section id='architecture'>
						<SectionHeading eyebrow='Architecture' title='Built not to drop events'>
							Writes go through RabbitMQ, reads go straight to the logger. Only the broker and the dashboard face the
							internet; everything that touches your data stays on an internal network.
						</SectionHeading>

						<Pipeline />
					</Section>

					<Section id='quick-start'>
						<SectionHeading eyebrow='Quick start' title='Up and running in five minutes'>
							Start the stack, sign in with GitHub, create an API key, and send your first event.
						</SectionHeading>

						<div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
							<div className='flex flex-col gap-3'>
								<h3 className='flex items-center gap-2 text-sm font-medium'>
									<StepNumber n={1} />
									Run the server
								</h3>
								<CodeBlock title='terminal' code={serverCode} />
								<p className='text-sm text-muted-foreground'>
									Caddy terminates TLS; MongoDB and RabbitMQ come with the compose file. See the{' '}
									<a href={`${DOCS_URL}/self-hosting.html`} className='text-primary underline-offset-4 hover:underline'>
										self-hosting guide
									</a>{' '}
									for production.
								</p>
							</div>

							<div className='flex flex-col gap-3'>
								<h3 className='flex items-center gap-2 text-sm font-medium'>
									<StepNumber n={2} />
									Instrument your app
								</h3>
								<CodeBlock title='app.ts' code={sdkCode} />
							</div>
						</div>
					</Section>

					<Section>
						<div className='relative flex flex-col items-start justify-between gap-6 overflow-hidden rounded-lg border bg-card p-8 md:flex-row md:items-center md:p-10'>
							<div className='bg-grid pointer-events-none absolute inset-0 opacity-60' aria-hidden />
							<div className='absolute inset-y-0 left-0 w-1 bg-primary' aria-hidden />
							<div className='relative'>
								<h2 className='text-2xl font-semibold tracking-tight'>Already running Logwolf?</h2>
								<p className='mt-2 text-muted-foreground'>Sign in with GitHub to open this instance's dashboard.</p>
							</div>
							<Button asChild size='lg' className='relative'>
								<Link to='/dashboard'>
									Open dashboard
									<ArrowRight />
								</Link>
							</Button>
						</div>
					</Section>
				</main>

				<footer>
					<div className='mx-auto flex max-w-6xl flex-col gap-4 px-6 py-8 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between'>
						<span className='flex items-center gap-2'>
							<LogoMark className='size-4' />
							Logwolf · GNU GPL v3
						</span>
						<nav className='flex gap-5'>
							<a href={DOCS_URL} className='hover:text-foreground'>
								Docs
							</a>
							<a href={GITHUB_URL} className='hover:text-foreground'>
								GitHub
							</a>
							<a href={NPM_URL} className='hover:text-foreground'>
								npm
							</a>
						</nav>
					</div>
				</footer>
			</div>
		</ThemeProvider>
	);
}
