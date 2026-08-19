import { createContext, useContext, useEffect } from 'react';
import { useLocalStorage } from 'usehooks-ts';

type Theme = 'dark' | 'light' | 'system';

export const THEME_STORAGE_KEY = 'logwolf-ui-theme';
export const DEFAULT_THEME: Theme = 'system';

type ThemeProviderState = {
	theme: Theme;
	setTheme: (theme: Theme) => void;
	toggleTheme: () => void;
};

const initialState: ThemeProviderState = {
	theme: DEFAULT_THEME,
	setTheme: () => null,
	toggleTheme: () => null,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

type ThemeProviderProps = {
	children: React.ReactNode;
	defaultTheme?: Theme;
	storageKey?: string;
};

export function ThemeProvider({
	children,
	defaultTheme = DEFAULT_THEME,
	storageKey = THEME_STORAGE_KEY,
	...props
}: ThemeProviderProps) {
	const [theme, setTheme] = useLocalStorage(storageKey, defaultTheme);

	useEffect(() => {
		const root = globalThis.document.documentElement;

		root.classList.remove('light', 'dark');

		if (theme === 'system') {
			const systemTheme = globalThis.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

			root.classList.add(systemTheme);
			return;
		}

		root.classList.add(theme);
	}, [theme]);

	const value = {
		theme,
		setTheme,
		toggleTheme: () => setTheme(theme === 'light' ? 'dark' : 'light'),
	};

	return (
		<ThemeProviderContext.Provider {...props} value={value}>
			{children}
		</ThemeProviderContext.Provider>
	);
}

export const useTheme = () => {
	const context = useContext(ThemeProviderContext);

	if (context === undefined) throw new Error('useTheme must be used within a ThemeProvider');

	return context;
};
