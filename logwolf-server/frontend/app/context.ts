import type { LogwolfEvent } from '@logwolf/client-js';
import { createContext } from 'react-router';

export const eventContext = createContext<LogwolfEvent | null>(null);
