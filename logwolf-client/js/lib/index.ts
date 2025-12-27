export function hello() {
	return 'world';
}

export * from './client';

import { Logwolf } from './client';
export default Logwolf;
