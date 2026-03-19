# JS SDK reference

The Logwolf JS SDK ships as `@logwolf/client-js` on npm. It handles event creation, sampling, and delivery to your Logwolf instance.

## Installation

```bash
npm install @logwolf/client-js
```

## Initialisation

```ts
import Logwolf from '@logwolf/client-js';

const logwolf = new Logwolf({
	url: 'https://logs.your-domain.com/api/',
	apiKey: process.env.LOGWOLF_API_KEY,
	sampleRate: 0.5,
	errorSampleRate: 1,
});
```

Configuration is validated at construction time using Zod. If a required field is missing or malformed, the constructor throws immediately.

### Configuration options

| Option            | Type     | Required | Description                                                                                                    |
| ----------------- | -------- | -------- | -------------------------------------------------------------------------------------------------------------- |
| `url`             | `string` | ✅       | Base URL of your Logwolf instance, including `/api/`. Must be a valid URL.                                     |
| `apiKey`          | `string` | ✅       | API key generated from the dashboard. Must start with `lw_`.                                                   |
| `sampleRate`      | `number` | —        | Fraction of `info` and `warning` events to send. `1` = all, `0.5` = half, `0` = none. Defaults to sending all. |
| `errorSampleRate` | `number` | —        | Fraction of `error` events to send. `critical` events always bypass sampling. Defaults to sending all.         |

## Creating events

Events are created using the `LogwolfEvent` class. Instantiating a `LogwolfEvent` starts a stopwatch — the duration is automatically computed when the event is serialised.

```ts
import { LogwolfEvent } from '@logwolf/client-js';

const event = new LogwolfEvent({
	name: 'checkout.completed',
	severity: 'info',
	tags: ['payments', 'stripe'],
});
```

### Constructor options

| Option     | Type                                           | Required | Description                                                                             |
| ---------- | ---------------------------------------------- | -------- | --------------------------------------------------------------------------------------- |
| `name`     | `string`                                       | ✅       | Event identifier. Use a consistent naming convention, e.g. `resource.action`.           |
| `severity` | `'info' \| 'warning' \| 'error' \| 'critical'` | ✅       | Event severity level.                                                                   |
| `tags`     | `string[]`                                     | ✅       | Array of tags for grouping and filtering. Duplicates are removed at serialisation time. |
| `data`     | `Record<string, unknown>`                      | —        | Initial key/value payload. Can be extended with `.set()`.                               |

### Attaching data

```ts
event.set('userId', '123');
event.set('orderId', 'ord_abc');
event.set('amount', 9900);
```

### Adding tags

```ts
event.addTag('checkout');
event.addTag('stripe');
```

### Retrieving data

```ts
const userId = event.get('userId');
```

### Changing severity

```ts
event.setSeverity('warning');
```

## Sending events

### `capture(event)`

Sends an event subject to sampling. This is the method you'll use in most cases.

```ts
await logwolf.capture(event);
```

Sampling behaviour:

- `info` and `warning` events are sampled at `sampleRate`
- `error` events are sampled at `errorSampleRate`
- `critical` events always bypass sampling and are always sent

### `create(event)`

Sends an event unconditionally, bypassing `sampleRate` and `errorSampleRate`. Use this when you explicitly want every event to be recorded regardless of sampling configuration.

```ts
await logwolf.create(event);
```

## Duration tracking

`LogwolfEvent` acts as a stopwatch. The duration is measured from object instantiation to the moment `capture()` or `create()` is called.

```ts
const event = new LogwolfEvent({
	name: 'db.query',
	severity: 'info',
	tags: ['database'],
});

const result = await db.query('SELECT ...');

event.set('rows', result.length);
await logwolf.capture(event); // duration = time since event was instantiated
```

The `duration` field appears in milliseconds in the dashboard and in the event payload.

## Retrieving events

### `getAll(pagination?)`

Fetches a paginated list of events from your Logwolf instance.

```ts
const events = await logwolf.getAll({ page: 1, pageSize: 20 });
```

Returns an array of `LogwolfEventData` objects.

### `getOne(id)`

Fetches a single event by ID. Note: this currently calls `getAll()` and scans in memory. For high-volume deployments, prefer using the dashboard or filtering by ID server-side.

```ts
const event = await logwolf.getOne('66f1a2b3c4d5e6f7a8b9c0d1');
```

## Deleting events

### `delete(dto)`

Deletes an event by ID.

```ts
await logwolf.delete({ id: '66f1a2b3c4d5e6f7a8b9c0d1' });
```

## Event severity guide

| Severity   | When to use                                                                                         |
| ---------- | --------------------------------------------------------------------------------------------------- |
| `info`     | Normal operations — user actions, background jobs, feature usage                                    |
| `warning`  | Something unexpected happened but the operation succeeded — retries, degraded paths, unusual inputs |
| `error`    | The operation failed — exceptions, failed requests, unhandled states                                |
| `critical` | Requires immediate attention — data corruption, security events, service outages                    |

`critical` events bypass sampling entirely and are always delivered.

## Usage patterns

### Wrapping an async operation

```ts
async function processOrder(orderId: string) {
	const event = new LogwolfEvent({
		name: 'order.process',
		severity: 'info',
		tags: ['orders'],
		data: { orderId },
	});

	try {
		const result = await doWork(orderId);
		event.set('result', result.status);
		await logwolf.capture(event);
	} catch (err) {
		event.setSeverity('error');
		event.set('error', err instanceof Error ? err.message : String(err));
		await logwolf.capture(event);
	}
}
```

### Capturing errors in a global handler

```ts
process.on('uncaughtException', async (err) => {
	const event = new LogwolfEvent({
		name: 'process.uncaughtException',
		severity: 'critical',
		tags: ['process'],
	});
	event.set('error', err.message);
	event.set('stack', err.stack);
	await logwolf.create(event);
});
```

### Next.js middleware

```ts
import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';
import Logwolf, { LogwolfEvent } from '@logwolf/client-js';

const logwolf = new Logwolf({
	url: process.env.LOGWOLF_URL!,
	apiKey: process.env.LOGWOLF_API_KEY!,
	sampleRate: 0.1,
	errorSampleRate: 1,
});

export function middleware(request: NextRequest) {
	const event = new LogwolfEvent({
		name: 'http.request',
		severity: 'info',
		tags: ['http'],
		data: {
			method: request.method,
			path: request.nextUrl.pathname,
		},
	});

	logwolf.capture(event).catch(() => {});

	return NextResponse.next();
}
```

## TypeScript types

The SDK is written in TypeScript and ships with full type definitions.

```ts
import type {
	LogwolfConfig, // Constructor options for `new Logwolf()`
	LogwolfEventDTO, // Constructor options for `new LogwolfEvent()`
	LogwolfEventData, // Shape of events returned by `getAll()` / `getOne()`
	Severity, // 'info' | 'warning' | 'error' | 'critical'
	Pagination, // { page: number; pageSize: number }
	DeleteLogwolfEventDTO, // { id: string }
} from '@logwolf/client-js';
```
