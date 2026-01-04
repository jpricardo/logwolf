import type { LogwolfEvent } from '@jpricardo/logwolf-client-js';
import { createContext } from 'react-router';

export const eventContext = createContext<LogwolfEvent | null>(null);
