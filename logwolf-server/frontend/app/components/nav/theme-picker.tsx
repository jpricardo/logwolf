import { Moon, Sun, SunMoon } from 'lucide-react';
import { useEffect, useState } from 'react';

import { DEFAULT_THEME, useTheme } from '~/store/theme-provider';

import { Button } from '../ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../ui/dropdown-menu';

const iconMap = {
	dark: <Moon />,
	light: <Sun />,
	system: <SunMoon />,
} as const;

export function ThemePicker() {
	const { theme, setTheme } = useTheme();

	// The stored theme is unknowable on the server, so render the default icon
	// until mount to keep the server and client markup identical. The document
	// theme itself is applied before first paint by the script in root.tsx.
	const [mounted, setMounted] = useState(false);
	useEffect(() => {
		setMounted(true);
	}, []);

	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button size='icon' variant='ghost'>
					{iconMap[mounted ? theme : DEFAULT_THEME]}
				</Button>
			</DropdownMenuTrigger>

			<DropdownMenuContent>
				<DropdownMenuItem onClick={() => setTheme('light')}>Light</DropdownMenuItem>
				<DropdownMenuItem onClick={() => setTheme('dark')}>Dark</DropdownMenuItem>
				<DropdownMenuItem onClick={() => setTheme('system')}>System</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
