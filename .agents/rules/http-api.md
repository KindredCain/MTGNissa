# HTTP API rules

## Requests and contracts

- Use `snake_case` for API fields, path parameters, query parameters, and frontend API action members defined by the contract. This does not prescribe naming for all frontend code.
- Keep decoding and validation behavior consistent through shared HTTP-layer helpers. Reject unknown JSON fields, undeclared query parameters, and trailing JSON values.
- Keep API contracts and callers synchronized when an interface changes. Update `docs/api-contract.md` and `docs/openapi.json` as applicable, regenerate frontend API types, and run the contract checks documented in `web/README.md`.

## Responses and errors

- Return every HTTP response through the shared `internal/httpresponse` helpers. Do not call Gin JSON response methods directly from handlers.
- Use `net/http` status constants at call sites; do not use numeric HTTP status literals.
- Keep the top-level response envelope stable: `status` is the actual HTTP status, concrete successful payloads are placed in `data`, `error` is `null` for success, and `data` is `null` for errors. Include `request_id` in every response.
- Represent HTTP errors with a stable machine-readable `error.code` and a client-safe `error.message`. Route validation failures, missing routes, internal failures, and panic recovery through the same error envelope.
- Treat a successfully queried background task as an HTTP success even when the task stage is `failed`; place the task state and task execution error inside `data`.
- Define HTTP request and response DTOs in the HTTP layer. Do not add HTTP JSON tags to internal domain or persistence models.
- Keep business errors independent of HTTP. Use `internal/apperror` for application errors and centralize their HTTP status and response mapping in `internal/httpresponse`.

## Request logging and Context

- Record request errors at the HTTP boundary without duplicating the same error in lower layers. Keep underlying causes, SQL, stack traces, and internal filesystem paths in logs rather than client responses.
- Pass the request Context to Services, queries, and transactions so deadlines and cancellation propagate. Background tasks use their own lifecycle Context.
