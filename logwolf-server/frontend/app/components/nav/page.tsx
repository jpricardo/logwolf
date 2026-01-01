import { AppHeader } from './app-header';

type Props = { title: string; children: React.ReactNode };
export function Page({ title, children }: Props) {
	return (
		<div className='flex flex-col gap-4 w-full'>
			<AppHeader title={title} />

			<div className='w-full'>{children}</div>
		</div>
	);
}
