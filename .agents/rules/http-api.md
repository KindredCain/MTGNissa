# HTTP API responses

- Return every HTTP response through the shared `internal/httpresponse` helpers. Do not call Gin JSON response methods directly from handlers.
- Use `net/http` status constants at call sites; do not use numeric HTTP status literals.
- Keep the top-level response envelope stable: `status` is the actual HTTP status, concrete successful payloads are placed in `data`, `error` is `null` for success, and `data` is `null` for errors. Include `request_id` in every response.
- Represent HTTP errors with a stable machine-readable `error.code` and a client-safe `error.message`. Route validation failures, missing routes, internal failures, and panic recovery through the same error envelope.
- Treat a successfully queried background task as an HTTP success even when the task stage is `failed`; place the task state and task execution error inside `data`.
- Define HTTP request and response DTOs in the HTTP layer. Do not add HTTP JSON tags to internal domain or persistence models.
