import { useEffect } from 'react';
import { toast } from 'sonner';

/**
 * Every intent on the project settings action answers with the same shape, so
 * each section can type its own fetcher without importing the route module.
 */
export type SettingsActionResult = { error?: string; success?: string } | null;

/** Announces a settings change once, when the fetcher comes back with one. */
export function useSuccessToast(result: SettingsActionResult | undefined) {
	useEffect(() => {
		if (result?.success) toast(result.success);
	}, [result]);
}
