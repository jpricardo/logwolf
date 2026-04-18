import Logwolf, { LogwolfEvent } from '@logwolf/client-js';

export const logwolf = new Logwolf({
	apiKey: process.env.API_KEY!,
	url: process.env.API_URL!,
	sampleRate: 0.5,
	errorSampleRate: 1,
	flushIntervalMs: 10_000,
	maxBatchSize: 100,
	maxQueueSize: 1000,
	requestTimeoutMs: 500,
	retryDelaysMs: [1000, 2000, 5000],
});

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
