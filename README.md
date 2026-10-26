# Harness

> **What if you had an agent that thinks like you, codes exactly like you, and does things exactly how you would?**
>
> Harness gives your AI agents your actual work experience—turning your past architectural decisions, solved production bugs, and hard-earned engineering instincts into a continuous, active context layer.

Harness is a lightweight, local-first [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server built in Go. Instead of letting coding assistants start from scratch on every prompt, Harness equips them with your historical track record, technical trade-offs, and operational tools so they write software the way you actually build it.

---

## Give Your Agent Your Engineering Experience

Most AI coding assistants suffer from a total lack of historical context:
* They don't know the race conditions you solved in your ledger service six months ago.
* They don't know your team strictly avoids certain ORMs or mandates idempotency keys on payment webhooks.
* They default to generic boilerplate that violates your production constraints.

Harness bridges this gap by acting as a cognitive and operational harness:

- **Institutional Developer Memory:** Automatically records and indexes the problems you tackle, the solutions you deploy, and the trade-offs you accept across all your projects.
- **Architectural & Style Alignment:** Injects your battle-tested patterns, concurrency rules, and conventions directly into the agent’s prompt loop.
- **Operational Precision:** Supplies targeted, token-efficient tools for schema inspection, test running, and failure diagnostics.
- **Zero-Friction Deployment:** Runs in seconds via Docker Compose or as a standalone Go binary, backed by an embedded SQLite datastore.

## Database Migrations

Schema changes are managed with [Goose](https://github.com/pressly/goose) SQL migrations in `db/migrations`. Install the pinned Goose CLI once (Go must be installed):

```sh
make goose-install
```

Make sure `$(go env GOPATH)/bin` is on your `PATH` so the `goose` command can be found.

Create a migration:

```sh
make migrate-create name=create_example_table
```

Edit the generated migration's `-- +goose Up` section to apply the change and its `-- +goose Down` section to reverse it. Then use:

```sh
make migrate-status
make migrate-up
make migrate-down
```

Commands use `./harness.db` by default. Override the database path when needed:

```sh
make migrate-up DB_PATH=/path/to/harness.db
```

For running migrations against the SQLite database inside Docker:

```sh
make docker-migrate-status
make docker-migrate-up
make docker-migrate-down
```

`make docker-up` runs `docker-migrate-up` before bringing up the container stack.

sqlc reads the same migration directory configured in `sqlc.yml`; Goose applies migrations to the database, while sqlc generates Go code from the schema and query SQL.
