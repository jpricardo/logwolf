# Changelog

## 2.0.0

Needs a Logwolf server with `GET /logs/:id`: the multi-tenancy release or later. Against an older server, `getOne` always resolves to `undefined`.

### Breaking

- **`getOne(id)` asks the server for that event** (`GET /logs/:id`) instead of fetching the latest page and searching it. It now finds an event of any age; it used to miss anything older than the newest 20. It resolves to `undefined` when the key's project has no event with that id, and throws on any other failure, as the other methods do. It needs a key with the `read` scope.
- **`getAll()` refuses a page the server would.** `pageSize` must be a whole number up to `MAX_PAGE_SIZE` (100) and `page` one up to `MAX_PAGE` (1,000,000). Anything else throws a `ZodError` before a request is made. A fractional or oversized `pageSize` used to be sent as it was.

### Added

- `MAX_PAGE_SIZE` and `MAX_PAGE`, exported.

### Fixed

- `getAll()`'s response was typed as the DOM's `Event[]` before parsing.

## 1.1.1

- Handle base URLs with a path (`https://example.com/api`).
