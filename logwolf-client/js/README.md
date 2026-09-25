# @logwolf/client-js

The JavaScript client for [Logwolf](https://github.com/jpricardo/logwolf), a self-hosted logging platform. It captures events in the browser or in Node, samples them, and delivers them to your Logwolf instance in batches, with retries.

```bash
npm install @logwolf/client-js
```

## Quick start

```ts
import Logwolf, { LogwolfEvent } from '@logwolf/client-js';

const logwolf = new Logwolf({
	url: 'https://logs.your-domain.com/api/',
	apiKey: process.env.LOGWOLF_API_KEY!, // an lw_ key from the dashboard's Keys page
	flushIntervalMs: 5000,
	maxBatchSize: 20,
	maxQueueSize: 500,
	retryDelaysMs: [1000, 3000, 10000],
	requestTimeoutMs: 10000,
});

const event = new LogwolfEvent({ name: 'checkout.completed', severity: 'info', tags: ['payments'] });
event.set('orderId', 'ord_123');
logwolf.capture(event); // returns at once; delivery is batched in the background

await logwolf.flush(); // before the process exits, so nothing queued is lost
```

`capture()` is synchronous and never throws: events are queued and sent in batches. `create()` sends one event now and awaits the server. `getAll()`, `getOne()` and `delete()` read and delete events; they need a key with the `read` or `delete` scope, which a key only has if it was created with them.

Severity is one of `info`, `warning`, `error` or `critical`.

## Documentation

- [SDK reference](https://github.com/jpricardo/logwolf/blob/main/docs/sdk/js.md): every option and method.
- [Changelog](https://github.com/jpricardo/logwolf/blob/main/logwolf-client/js/CHANGELOG.md): 2.0.0 changed `getOne` and `getAll`'s pagination, and needs a recent Logwolf server.
