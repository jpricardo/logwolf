export function hello() {
	return 'world';
}

export * from './client';
export * from './event';

import { Logwolf } from './client';
export default Logwolf;
