import Logwolf, { LogwolfEvent } from '@jpricardo/logwolf-client-js';

export const logwolf = new Logwolf({ url: process.env.API_URL!, sampleRate: 0.5, errorSampleRate: 1 });

export function injectRequest(ev: LogwolfEvent, request: Request) {
	ev.set('request', {
		cache: request.cache,
		credentials: request.credentials,
		destination: request.destination,
		headers: Object.fromEntries(request.headers.entries()),
		integrity: request.integrity,
		keepalive: request.keepalive,
		method: request.method,
		mode: request.mode,
		referrer: request.referrer,
		redirect: request.redirect,
		url: request.url,
	});
}

export function injectResponse(ev: LogwolfEvent, response: Response) {
	ev.set('response', {
		headers: Object.fromEntries(response.headers.entries()),
		status: response.status,
		statusText: response.statusText,
		type: response.type,
		url: response.url,
	});
}
