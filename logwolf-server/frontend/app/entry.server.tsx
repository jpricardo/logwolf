import { PassThrough } from 'node:stream';

import { LogwolfEvent } from '@jpricardo/logwolf-client-js';
import { createReadableStreamFromReadable } from '@react-router/node';
import { isbot } from 'isbot';
import type { RenderToPipeableStreamOptions } from 'react-dom/server';
import { renderToPipeableStream } from 'react-dom/server';
import type { AppLoadContext, EntryContext, HandleErrorFunction } from 'react-router';
import { ServerRouter } from 'react-router';
import { injectRequest, logwolf } from './lib/logwolf';

export const streamTimeout = 5_000;

export const handleError: HandleErrorFunction = function (error, { request, context, params }) {
	if (request.signal.aborted) return;

	const event = new LogwolfEvent({ name: 'Server Error', severity: 'critical', tags: ['logwolf_frontend', 'server'] });
	event.set('error', error);
	event.set('context', context);
	event.set('params', params);
	injectRequest(event, request);
	logwolf.capture(event);
};

export default function handleRequest(
	request: Request,
	responseStatusCode: number,
	responseHeaders: Headers,
	routerContext: EntryContext,
	loadContext: AppLoadContext,
	// If you have middleware enabled:
	// loadContext: RouterContextProvider
) {
	// https://httpwg.org/specs/rfc9110.html#HEAD
	if (request.method.toUpperCase() === 'HEAD') {
		return new Response(null, {
			status: responseStatusCode,
			headers: responseHeaders,
		});
	}

	return new Promise((resolve, reject) => {
		let shellRendered = false;
		let userAgent = request.headers.get('user-agent');

		// Ensure requests from bots and SPA Mode renders wait for all content to load before responding
		// https://react.dev/reference/react-dom/server/renderToPipeableStream#waiting-for-all-content-to-load-for-crawlers-and-static-generation
		let readyOption: keyof RenderToPipeableStreamOptions =
			(userAgent && isbot(userAgent)) || routerContext.isSpaMode ? 'onAllReady' : 'onShellReady';

		// Abort the rendering stream after the `streamTimeout` so it has time to
		// flush down the rejected boundaries
		let timeoutId: ReturnType<typeof setTimeout> | undefined = setTimeout(() => abort(), streamTimeout + 1000);

		const { pipe, abort } = renderToPipeableStream(<ServerRouter context={routerContext} url={request.url} />, {
			[readyOption]() {
				shellRendered = true;
				const body = new PassThrough({
					final(callback) {
						// Clear the timeout to prevent retaining the closure and memory leak
						clearTimeout(timeoutId);
						timeoutId = undefined;
						callback();
					},
				});
				const stream = createReadableStreamFromReadable(body);

				responseHeaders.set('Content-Type', 'text/html');

				pipe(body);

				resolve(
					new Response(stream, {
						headers: responseHeaders,
						status: responseStatusCode,
					}),
				);
			},
			onShellError(error: unknown) {
				reject(error);
			},
			onError(error: unknown) {
				responseStatusCode = 500;
				// Log streaming rendering errors from inside the shell.  Don't log
				// errors encountered during initial shell rendering since they'll
				// reject and get logged in handleDocumentRequest.
				if (shellRendered) {
					console.error(error);
				}
			},
		});
	});
}
