import { useRouteLoaderData } from 'react-router';

import type { loader as layoutLoader } from '../pages/layout';

export function useCsrfToken() {
	const layoutData = useRouteLoaderData<typeof layoutLoader>('pages/layout');
	const csrfToken = layoutData?.csrfToken ?? '';

	return csrfToken;
}
