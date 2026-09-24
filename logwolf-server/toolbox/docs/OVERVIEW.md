# Toolbox — Overview

## Purpose

Shared Go library used by Broker, Listener, and Logger. It centralises data models, MongoDB collection helpers, RabbitMQ connection and queue setup, and JSON utilities so that each service stays thin.

Toolbox is a local module in the Go workspace (`logwolf-server/go.work`) and is imported as `logwolf-toolbox/...`. It is never published to an external registry.

## Package layout

```
toolbox/
├── data/
│   ├── models.go    # Models struct, LogEntry, APIKey, Settings types + CRUD methods
│   ├── apikey.go    # APIKey model, key generation, validation
│   ├── settings.go  # Per-project retention settings, index management
│   ├── project.go   # Project and ProjectMember models, membership queries
│   ├── migrate.go   # Startup migration of pre-multi-tenancy data
│   └── log.go       # Log entry type aliases
├── event/
│   ├── event.go     # Exchange + queue declarations
│   ├── emitter.go   # RabbitMQ message publisher
│   ├── consumer.go  # RabbitMQ message consumer: manual acks, retries
│   └── logger_client.go # The consumer's one reused RPC connection to Logger
├── rabbitmq/
│   └── connect.go   # RabbitMQ connection initialisation
└── json/
    └── helpers.go   # JSON encode/decode utilities
```

## `data` package

### `Models` struct

The central database accessor. Services initialise one `Models` value and pass it around:

```go
models := data.New(mongoClient)
// then use:
models.LogEntry.Insert(entry)
models.LogEntry.AllLogs(queryParams)
models.LogEntry.DeleteOne(filter)
models.APIKey.Insert(key)
models.Settings.Get()
```

### `LogEntry`

Represents a single log record in MongoDB. Key fields: `Name`, `Data`, `Severity`, `Tags`, `Duration`, `CreatedAt`, `UpdatedAt`.

### `APIKey`

Stores API key metadata: project ID, bcrypt hash, the key's first 10 characters in clear as `prefix` (`lw_` + 7), active flag, and created/revoked timestamps. `GenerateAPIKey` builds `lw_` + 32 random bytes in base64url, 46 characters in all.

`Scopes` lists what the key may do on the public `/logs` routes: `ingest`, `read`, `delete` (`AllScopes`). `NormalizeScopes` validates, deduplicates and orders them; an empty list becomes `DefaultScopes`, which is `ingest` alone. A key stored before scopes existed has none, and every read path fills in `AllScopes` and sets `Legacy`, which is not stored, so it keeps the full access it always had.

`ValidateAPIKey` refuses anything not shaped like that without a query. For the rest, it fetches only the active keys that share the prefix, normally exactly one, and bcrypts those. Its cost does not grow with the number of keys across projects. `EnsureAPIKeyIndexes` creates the `prefix` index that lookup uses; Logger calls it on startup.

`SaveAPIKey` sets the key's `ID` to the one it was stored under. `RevokeAPIKey` takes the project as well as the id and matches both, so an id from another project is `ErrKeyNotFound`, the same as one that never existed. Only Logger calls these; the Broker reaches them over RPC with the `RPC*APIKey*` argument types.

### `Settings`

Manages per-project settings documents (currently: retention in days), keyed by `(project_id, key)`. `LowersRetention(from, to)` says whether a change shortens retention, 0 (forever) being the longest; the Broker lets only owners make such a change.

### Project ids

Every `project_id` (`LogEntry`, `APIKey`, settings, `ProjectMember`) is a `primitive.ObjectID`, the type of `projects._id`, and every `Models` method takes one. A filter with the wrong type matches nothing without an error, so there is only one. RPC argument types (`QueryParams`, `ProjectArgs`, `RetentionArgs`, the `RPC*` structs) still carry the id as a hex string; Logger parses it in its RPC layer. JSON output is unchanged: an ObjectID marshals to its hex string.

### Projects and members (`project.go`)

A project's `Slug` is a display label, derived from its name when it is created and fixed after that. It is not unique and nothing looks a project up by it, so creating a project never reveals that another user has one with the same slug. The migration's `Default` project is found by its `Default` flag (`default` in BSON), which a partial unique index (`unique_default`) allows on one project only. `RenameProject` changes the name alone.

Three operations run in MongoDB transactions, so MongoDB must run as a replica set (a single member is enough, and that is how `docker-compose.yml` and the integration tests run it):

- `CreateProjectWithOwner` inserts a project and its first owner's membership as one unit. A project without an owner could never be reached, since only owners add members.
- `DeleteProject` removes the project's API keys, settings, members and the project itself as one unit. A failure part-way rolls the whole thing back. The logs are left out, since a big project's would outlast the transaction; `PurgeProjectLogs` deletes them afterwards and refuses (`ErrProjectExists`) for a project that still exists.
- `RemoveProjectMember` and `UpdateProjectMemberRole` count the owners and remove or demote the member in one transaction (`changeMembers`), and write to the project document first (`members_updated_at`). A transaction on its own would still let two concurrent removals or demotions of different owners both pass the count. The write to a shared document forces a write conflict, `WithTransaction` retries the loser, and the retry sees `ErrLastOwner`. Setting the role a member already holds is a no-op.

GitHub logins are case-insensitive, so `project_members` stores them normalized (`NormalizeGithubLogin`: trimmed, lowercase). `InsertProjectMember`, `RemoveProjectMember`, `UpdateProjectMemberRole`, `MemberRole` and `GetProjectsForUser` all normalize the login they are given. That lets a login typed in any casing match the one GitHub returns at sign-in, and the unique `(project_id, github_login)` index refuses case-only duplicates.

`ProjectExists` answers whether a project id names a project. Logger uses it to refuse events for deleted projects, and `DeleteOrphanedLogs` (in `models.go`) removes the ones that got through: logs whose `project_id` matches no project and that are older than a minute, so it never races a project being created. It skips logs with no or an empty `project_id`, which belong to the startup migration. A `project_id` still stored as a string is compared by its text, so a live project's logs that `ConvertProjectIDs` has not reached yet are never taken for orphans. It, `PurgeProjectLogs` and `DeleteExpiredLogs` all delete in batches of 10,000, each with its own 30s timeout, so a project with millions of logs is purged over as long as it takes rather than failing on one `DeleteMany`.

### Startup migration (`migrate.go`)

Adopts data written before projects existed. Logger calls it on every start; Broker and Listener never do.

| Function                         | Description                                                                             |
| -------------------------------- | --------------------------------------------------------------------------------------- |
| `ConvertProjectIDs`              | Rewrites hex string `project_id`s in `logs`, `api_keys`, `settings` to ObjectIDs        |
| `MarkDefaultProject`             | Flags the Default project of earlier builds, then drops the old unique slug index       |
| `CountOrphanedDocuments`         | Counts `logs`, `api_keys`, and `settings` documents with no project ID                  |
| `MigrateOrphansToDefaultProject` | Adopts those documents into the `Default` project, creating it and its owners if needed |
| `EnsureDefaultProjectOwners`     | Gives an ownerless `Default` project its owners, promoting existing members if listed   |
| `NormalizeMemberLogins`          | Lowercases stored member logins, merging case-only duplicates into the higher role      |
| `DropLegacyTTLIndex`             | Removes the global TTL index that predates per-project retention                        |
| `ParseGithubLogins`              | Splits a comma-separated allowlist into normalized logins (deduplicated ignoring case)  |

`MigrateOrphansToDefaultProject` returns a nil `*MigrationReport` when there is nothing to adopt, which is what makes repeated runs a no-op.

That is also why it can't be trusted to add owners on its own. If its owner step fails, or runs with an empty owner list, the next start has no orphans left and returns early. `EnsureDefaultProjectOwners` runs independently of the orphan count and only acts while `Default` has no owner. It returns a nil `*OwnerRepair` when there is no `Default` project or it already has an owner.

## `event` package

Declares the RabbitMQ topology used by all services:

- **Exchange**: `logs_topic` (topic type, durable)
- **Named queues**: durable, survive broker restarts
- **Random/exclusive queues**: temporary, used for one-off consumers

`emitter.go` wraps `amqp.Channel.Publish` for structured event publishing.  
`consumer.go` provides `NewConsumer` + `Listen`, the main loop used by Listener. It acknowledges a message only once Logger has stored the event or it has been dropped for good, and retries an unreachable Logger with back-off; see the [Listener overview](../../listener/docs/OVERVIEW.md#delivery-guarantees) for the rules. `logger_client.go` holds the one RPC connection it reuses for every event.

## `rabbitmq` package

Single `Connect(url string) (*amqp.Connection, error)` function with retry logic for startup ordering (RabbitMQ may not be ready when a service starts).

## `json` package

Lightweight wrappers around `encoding/json` used consistently across services for reading request bodies and writing responses.

## Key dependencies

| Dependency            | Role                  |
| --------------------- | --------------------- |
| `rabbitmq/amqp091-go` | RabbitMQ client       |
| `mongo-driver`        | MongoDB client        |
| `golang.org/x/crypto` | Secure key generation |

## Development

Toolbox has its own unit tests:

```bash
cd logwolf-server/toolbox && go test ./... -v
```

Because Toolbox is a library with no `main` package, it is not run or deployed independently — it is always compiled into the services that depend on it.
