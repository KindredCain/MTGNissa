# Backend design rules

## Layers and models

- Follow the dependency direction `httpapi -> business Service -> store`. Stores must not depend on Service packages; shared models must not depend on either layer.
- Keep HTTP protocol adaptation in `internal/httpapi`. Services own business rules and coordination across business capabilities. Stores own SQL, scanning and basic mapping, locking, persistence, and necessary query-result organization.
- Keep HTTP DTOs in `internal/httpapi`, shared business models and their query parameters and results in `internal/model`, persistence entities in `internal/appdata`, and import models in `internal/carddata`.
- Place types by responsibility, not merely because they cross a layer boundary. Store-specific query parameters and pagination positions may stay in `store` and be used by Services. Keep implementation-only scan rows, grouping keys, and cursor encoding structures in their owning implementation; do not move types into `model` solely for uniformity.
- Follow existing module patterns. Give shared abstractions a clear responsibility and actual reuse need; do not add layers or split packages speculatively.
- Reuse existing capabilities before adding dependencies. Add a third-party dependency only for a clear benefit, and avoid frameworks or tools with overlapping responsibilities.

## Dependencies and resources

- Inject dependencies explicitly through constructors and assemble them in the startup layer. Do not use global database variables.
- Create database pools centrally at startup and close them centrally during shutdown, not per request.
- Propagate the request Context into queries and transactions. Background tasks use a Context whose lifetime matches the task rather than the request that started it.

## SQL and queries

- Use `sqlx` for business data access through the owning database or transaction handle. Migrations, card-data loading, and health checks retain their established `database/sql` boundary.
- Prefer `goqu` when dynamic query composition is complex. Parameterized handwritten SQL is appropriate for fixed structures and a small number of controlled optional conditions; use `sqlx.In` when only the number of `IN` values changes.
- Choose SQL tools by composition complexity and maintenance cost, not endpoint names or statement length. Do not rewrite SQL into a Builder merely for uniformity.
- Bind input values as parameters. Resolve dynamic table names, columns, operators, and sort directions through internal controlled mappings; never interpolate user text directly into SQL.
- Prefer batch and set-based queries. Preserve existing combined queries during refactoring; avoid unnecessary per-item follow-up queries, reads, and repeated computation.
- Choose database and application work by end-to-end cost at realistic data sizes, considering latency, database CPU and temporary tables, application memory, network transfer, and database compatibility. Do not assume that fewer returned rows or moving more work into SQL makes a query faster.
- Materializing and reusing an intermediate ID set is allowed when it avoids repeated expensive matching. Assess its size, memory and transfer cost, and SQL parameter limits; keep endpoint pagination and current-page metadata loading intact. Do not classify an intermediate ID set alone as a defect.
- For performance-motivated query rewrites, compare execution plans and representative timings on supported database versions. Without that evidence, report a potential optimization rather than an established bottleneck or a required rewrite.

## Transactions and consistency

- Services decide which steps belong in a transaction. Use `store.AppTransactions.WithAppTx` to manage commit and rollback and the transaction-bound stores exposed by its `AppTx` callback. Stores do not commit a caller-owned transaction or switch transaction steps to an ordinary database connection.
- Commit related business changes and any persistent operation logs that require atomicity in the same transaction. Lock before reading and modifying state when serialization is required. When normalized business state has not changed, do not produce additional business updates, timestamps, or operation logs.
- Do not perform cross-database transactions or cross-database SQL JOINs. Complete independent external dependency reads and validation before a write transaction where possible; validation that depends on locked state belongs inside the transaction.
- Use the corresponding read snapshot for combined reads that require consistency. Keep each database's snapshot independent; do not imply a global snapshot across databases.
- Give caches and idempotency records an explicit owner, lifetime, and invalidation policy. Mark an idempotent operation complete only after its transaction commits. Caches do not replace authoritative data.

## Database migrations

- Add a new migration version for schema changes; do not rewrite migrations that have already been applied.
- Preserve the database compatibility range and migration ownership defined in `docs/architecture.md` and `docs/app-data-schema.md`.
