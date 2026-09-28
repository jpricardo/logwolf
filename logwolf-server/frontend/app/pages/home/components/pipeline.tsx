import { ArrowDown, ArrowRight } from 'lucide-react';
import { Fragment } from 'react';

type Stage = { name: string; role: string };

const publicStages: Stage[] = [
	{ name: 'SDK', role: 'Batches and retries in your app' },
	{ name: 'Broker', role: 'Validates the key, answers 202' },
];

const internalStages: Stage[] = [
	{ name: 'RabbitMQ', role: 'Persistent, confirmed queue' },
	{ name: 'Listener', role: 'Acks once the event is stored' },
	{ name: 'Logger', role: 'The only service with DB access' },
	{ name: 'MongoDB', role: 'Per-project retention' },
];

function Arrow() {
	return (
		<div className='flex items-center justify-center text-muted-foreground' aria-hidden>
			<ArrowDown className='size-4 xl:hidden' />
			<ArrowRight className='hidden size-4 xl:block' />
		</div>
	);
}

function Stages({ stages }: { stages: Stage[] }) {
	return stages.map((s, i) => (
		<Fragment key={s.name}>
			{i > 0 && <Arrow />}
			<div className='rounded-md border bg-card px-3 py-2.5 xl:min-h-17 xl:w-36'>
				<div className='font-medium'>{s.name}</div>
				<div className='text-xs text-muted-foreground'>{s.role}</div>
			</div>
		</Fragment>
	));
}

export function Pipeline() {
	return (
		<div className='flex flex-col items-stretch gap-2 text-sm xl:flex-row xl:items-center'>
			<Stages stages={publicStages} />
			<Arrow />

			<div className='relative flex flex-col gap-2 rounded-lg border border-dashed p-3 pt-7 xl:flex-row xl:items-center'>
				<span className='absolute top-2 left-3 font-mono text-[11px] text-muted-foreground'>internal network</span>
				<Stages stages={internalStages} />
			</div>
		</div>
	);
}
