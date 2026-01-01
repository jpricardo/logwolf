import { Separator } from '../ui/separator';
import { SidebarTrigger } from '../ui/sidebar';
import { ThemePicker } from './theme-picker';

type Props = { title: string };
export function AppHeader({ title }: Props) {
	return (
		<header className='flex h-(--header-height) shrink-0 items-center gap-2'>
			<div className='flex w-full items-center gap-1'>
				<SidebarTrigger />

				<Separator orientation='vertical' className='mx-2 data-[orientation=vertical]:h-4' />

				<div className='w-full flex flex-row items-center justify-between'>
					<span>{title}</span>

					<ThemePicker />
				</div>
			</div>
		</header>
	);
}
