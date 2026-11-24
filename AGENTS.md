# AI contributor instructions

## Design references

Before changing a module, read the relevant parts of `docs/architecture.md`, `docs/api-contract.md`, `docs/openapi.json`, `docs/app-data-schema.md`, and `docs/card-data-dictionary.md` as applicable. For frontend structure and interaction, also consult `web/README.md`, `design/README.md`, and the corresponding page specification. Identify and resolve conflicting guidance explicitly rather than silently choosing one interpretation.

## Required rules

Before creating or modifying Go code, read and follow `.agents/rules/go-style.md`.

Before creating or modifying backend code or database migrations, read and follow `.agents/rules/backend-design.md`.

Before creating or modifying HTTP endpoints, request handling, response handling, or API contracts, read and follow `.agents/rules/http-api.md`.

Before creating or modifying frontend code, frontend build configuration, or browser interaction behavior, read and follow `.agents/rules/frontend.md`.

## Changes and verification

- When an architectural boundary changes, update the relevant design documents and contributor rules together.
- Select formatting, static analysis, type checking, contract checks, and tests according to the affected code. Go checks include `gofmt`, `go vet`, and relevant `go test` packages; frontend checks use the existing Corepack/pnpm scripts documented in `web/README.md`.
- For behavior changes, cover normal, failure, and boundary cases according to risk. Do not mechanically add tests for formatting-only or low-impact changes.
- For review-only tasks, do not modify files or expand the task into implementation. Keep verification within the requested scope.
