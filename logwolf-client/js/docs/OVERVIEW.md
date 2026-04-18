# @logwolf/client-js — Overview

## Purpose

TypeScript/JavaScript client SDK for submitting events to and reading events from a Logwolf backend. Designed to be embedded in any web or Node.js application.

## Package info

| Field           | Value                          |
| --------------- | ------------------------------ |
| Package name    | `@logwolf/client-js`           |
| Current version | `1.1.1`                        |
| Entry point     | `dist/logwolf-client.js` (ESM) |
| Source          | `lib/`                         |
| Build tool      | Rollup + TypeScript            |

## Source layout

```
lib/
├── index.ts      # Public exports (Logwolf, LogwolfEvent, schemas)
├── client.ts     # Core Logwolf class — public API, batching, retry
├── event.ts      # LogwolfEvent helper class
└── schema.ts     # Zod schemas for config and API contracts
```

## Public API

```ts
const client = new Logwolf({
  baseUrl: 'https://your-logwolf.example.com',
  apiKey:  'lw_...',
  // optional
  flushInterval:    5000,   // ms between auto-flushes
  maxBatchSize:     100,
  sampleRate:       1.0,    // 0–1
  errorSampleRate:  1.0,
  timeout:          10000,  // fetch timeout in ms
});

client.capture({ name: 'user.signup', severity: 'INFO', data: { ... } });

await client.flush();    // force-send queued events
await client.destroy();  // flush + stop background timer

// Read-side (requires valid API key with read permission)
const events = await client.getAll({ page: 1, limit: 50 });
const event  = await client.getOne(id);
await client.delete(id);
```

## Event flow

1. `capture()` validates the event with Zod and pushes it to an in-memory queue.
2. A background timer (default 5 s) batches queued events and `POST /logs/batch`.
3. Failed requests retry with exponential back-off (up to 3 attempts).
4. If the queue exceeds `maxBatchSize`, the oldest events are evicted (FIFO).

## Key design decisions

- **Fire-and-forget capture**: `capture()` is synchronous; actual delivery is asynchronous.
- **No singleton**: callers create their own `Logwolf` instances; multiple targets are supported.
- **Zod for runtime safety**: config and event payloads are validated at the boundary, so bad data surfaces early.
- **Sample rates**: general `sampleRate` and a separate `errorSampleRate` allow cheaper sampling of normal events while retaining all errors.

## Dependencies

| Dependency | Role                      |
| ---------- | ------------------------- |
| `zod`      | Runtime schema validation |

All dev dependencies (Rollup, Vitest, TypeScript, oxlint/oxfmt) are build-time only and not shipped.

## Development commands

```bash
npm test          # vitest (watch mode)
npm run coverage  # single run with coverage report
npm run build     # tsc + rollup → dist/
npm run lint      # oxlint
npm run format    # oxfmt
npm run typecheck # tsc --noEmit
```

## Relationship to the rest of Logwolf

The SDK talks directly to the **Broker** service (`POST /logs`, `POST /logs/batch`, `GET /logs`, `DELETE /logs`) using a Bearer token (`lw_` prefix). It has no knowledge of RabbitMQ, MongoDB, or any internal service — it only needs the public Broker URL and a valid API key.
