import { ChevronRight } from 'lucide-react';
import { Fragment } from 'react';
import { Link } from 'react-router';

import { useProjects } from '~/hooks/use-projects';

import { Separator } from '../ui/separator';
import { SidebarTrigger } from '../ui/sidebar';
import { ThemePicker } from './theme-picker';

export type Crumb = { label: string; to: string };

type Props = { title: string; parents?: Crumb[] };
export function AppHeader({ title, parents = [] }: Props) {
	const { currentProject } = useProjects();

	return (
		<header className='sticky top-0 z-20 flex h-14 shrink-0 items-center gap-2 border-b bg-background/85 px-4 backdrop-blur-sm md:px-6'>
			<SidebarTrigger className='-ml-1.5' />

			<Separator orientation='vertical' className='mx-1.5 data-[orientation=vertical]:h-4' />

			<nav aria-label='Breadcrumb' className='flex min-w-0 flex-1 items-center gap-1.5 text-sm'>
				{currentProject && (
					<>
						<span className='hidden truncate text-muted-foreground sm:inline'>{currentProject.name}</span>
						<ChevronRight className='hidden size-3.5 shrink-0 text-muted-foreground/60 sm:block' />
					</>
				)}

				{parents.map((p) => (
					<Fragment key={p.to}>
						<Link to={p.to} className='truncate text-muted-foreground transition-colors hover:text-foreground'>
							{p.label}
						</Link>
						<ChevronRight className='size-3.5 shrink-0 text-muted-foreground/60' />
					</Fragment>
				))}

				<span className='truncate font-medium'>{title}</span>
			</nav>

			<ThemePicker />
		</header>
	);
}
