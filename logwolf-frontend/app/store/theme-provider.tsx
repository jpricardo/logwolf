import { createContext, useContext, useEffect } from 'react';
import { useLocalStorage } from 'usehooks-ts';

type Theme = 'dark' | 'light' | 'system';

type ThemeProviderState = {
	theme: Theme;
	setTheme: (theme: Theme) => void;
	toggleTheme: () => void;
};

const initialState: ThemeProviderState = {
	theme: 'light',
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
	defaultTheme = 'system',
	storageKey = 'logwolf-ui-theme',
	...props
}: ThemeProviderProps) {
	const [theme, setTheme] = useLocalStorage(storageKey, defaultTheme);

	// TODO - Solve FOUC
	useEffect(() => {
		const root = window.document.documentElement;

		root.classList.remove('light', 'dark');

		if (theme === 'system') {
			const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

			root.classList.add(systemTheme);
			return;
		}

		root.classList.add(theme);
	}, [theme]);

	const value = {
		theme,
		setTheme: (theme: Theme) => {
			localStorage.setItem(storageKey, theme);
			setTheme(theme);
		},
		toggleTheme: () => {
			const newTheme = theme === 'light' ? 'dark' : 'light';
			localStorage.setItem(storageKey, newTheme);
			setTheme(newTheme);
		},
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
