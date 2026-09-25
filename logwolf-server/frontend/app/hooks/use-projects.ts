import { useRouteLoaderData } from 'react-router';

import type { loader as layoutLoader } from '../pages/layout';

/**
 * Projects the signed-in user belongs to, plus the one they are working in.
 * Both are already loaded by the layout every dashboard page renders inside,
 * so pages read them from there instead of asking the broker again.
 */
export function useProjects() {
	const layoutData = useRouteLoaderData<typeof layoutLoader>('pages/layout');

	return {
		projects: layoutData?.projects ?? [],
		currentProject: layoutData?.currentProject,
	};
}
