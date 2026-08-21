/**
 * Turns a free-text project name into a URL-safe slug.
 *
 * The output must satisfy the broker's `data.ValidSlug` regex,
 * `^[a-z0-9]+(?:-[a-z0-9]+)*$`, because whatever this returns is what gets
 * submitted. A name made only of punctuation slugifies to an empty string,
 * which the caller has to reject before hitting the API.
 */
export function slugify(name: string): string {
	return (
		name
			.normalize('NFKD')
			// NFKD splits accented letters into base + combining mark; dropping the
			// marks keeps the base letters instead of losing the whole character.
			.replaceAll(/[\u0300-\u036f]/g, '')
			.toLowerCase()
			.replaceAll(/[^a-z0-9]+/g, '-')
			.replaceAll(/^-+|-+$/g, '')
	);
}
