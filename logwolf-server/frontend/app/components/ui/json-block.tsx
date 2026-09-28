import { cn } from '~/lib/utils';

// Strings (and whether they are a key), literals and numbers in
// JSON.stringify's output; everything between them is punctuation.
const tokenPattern =
	/("(?:\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*")(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g;

function highlight(json: string) {
	const out: React.ReactNode[] = [];
	let last = 0;

	for (const m of json.matchAll(tokenPattern)) {
		if (m.index > last) out.push(json.slice(last, m.index));

		const [text, str, colon, literal] = m;
		if (str && colon) {
			out.push(
				<span key={m.index} className='text-foreground'>
					{str}
				</span>,
				colon,
			);
		} else if (str) {
			out.push(
				<span key={m.index} className='text-chart-2'>
					{str}
				</span>,
			);
		} else if (literal) {
			out.push(
				<span key={m.index} className='text-chart-4'>
					{literal}
				</span>,
			);
		} else {
			out.push(
				<span key={m.index} className='text-primary'>
					{text}
				</span>,
			);
		}

		last = m.index + text.length;
	}

	if (last < json.length) out.push(json.slice(last));
	return out;
}

export type Props = Omit<React.ComponentProps<'div'>, 'children'> & {
	data: Record<string | number | symbol, unknown>;
};

export function JSONBlock({ data, className, ...props }: Props) {
	const json = JSON.stringify(data, undefined, 2) ?? '';

	return (
		<div className={cn('overflow-auto rounded-lg border bg-card', className)} {...props}>
			<pre className='p-4 font-mono text-[13px] leading-6 text-muted-foreground'>
				<code>{highlight(json)}</code>
			</pre>
		</div>
	);
}
