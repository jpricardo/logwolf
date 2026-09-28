import { cn } from '~/lib/utils';

// A trailing comment is anything after "//" or "#" preceded by whitespace (or
// at the start of the line), so URLs like https://… are left alone.
const commentPattern = /(^|\s)(\/\/|#)\s.*$/;

function Line({ text }: { text: string }) {
	const match = commentPattern.exec(text);
	if (!match) return <>{text}</>;

	const start = match.index + match[1]!.length;
	return (
		<>
			{text.slice(0, start)}
			<span className='text-muted-foreground'>{text.slice(start)}</span>
		</>
	);
}

type Props = React.ComponentProps<'div'> & { title: string; code: string };
export function CodeBlock({ title, code, className, ...props }: Props) {
	return (
		<div className={cn('overflow-hidden rounded-lg border bg-card', className)} {...props}>
			<div className='border-b px-4 py-2 font-mono text-xs text-muted-foreground'>{title}</div>
			<pre className='overflow-x-auto p-4 font-mono text-[13px] leading-6'>
				<code>
					{code.split('\n').map((line, i) => (
						<div key={i}>
							<Line text={line} />
							{line === '' && ' '}
						</div>
					))}
				</code>
			</pre>
		</div>
	);
}
