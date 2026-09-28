import { cn } from '~/lib/utils';

import { AppHeader, type Crumb } from './app-header';

type Props = {
	title: string;
	/** Shown under the heading. */
	description?: React.ReactNode;
	/** Buttons on the heading's right, e.g. the page's primary action. */
	actions?: React.ReactNode;
	/** Pages above this one in the header's trail. */
	parents?: Crumb[];
	/** Replaces the plain-text heading, e.g. with badges next to it. */
	heading?: React.ReactNode;
	className?: string;
	children: React.ReactNode;
};
export function Page({ title, description, actions, parents, heading, className, children }: Props) {
	return (
		<div className='flex w-full flex-col'>
			<AppHeader title={title} parents={parents} />

			<div className={cn('mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-8 md:px-8', className)}>
				<div className='flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between'>
					<div className='flex min-w-0 flex-col gap-1.5'>
						{heading ?? <h1 className='truncate text-2xl font-semibold tracking-tight'>{title}</h1>}
						{description && <p className='text-sm text-muted-foreground'>{description}</p>}
					</div>

					{actions && <div className='flex shrink-0 items-center gap-2'>{actions}</div>}
				</div>

				{children}
			</div>
		</div>
	);
}
