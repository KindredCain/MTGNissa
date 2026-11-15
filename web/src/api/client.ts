import type { paths, components } from "./schema";

export type DTO<K extends keyof components["schemas"]> =
  components["schemas"][K];
type Operation<P extends keyof paths, M extends keyof paths[P]> = NonNullable<
  paths[P][M]
>;
type Payload<O> = O extends { responses: infer R }
  ? {
      [S in keyof R]: R[S] extends { content: { "application/json": infer E } }
        ? E extends { data: infer D }
          ? NonNullable<D>
          : never
        : never;
    }[Exclude<keyof R, "default">]
  : never;
type Body<O> = O extends {
  requestBody: { content: { "application/json": infer B } };
}
  ? B
  : never;
type Query<O> = O extends { parameters: { query?: infer Q } } ? Q : never;
type GetPath = {
  [P in keyof paths]: paths[P]["get"] extends undefined ? never : P;
}[keyof paths];
type PostPath = {
  [P in keyof paths]: paths[P]["post"] extends undefined ? never : P;
}[keyof paths];
export type QueryParams<P extends GetPath> = Query<Operation<P, "get">>;
type Options = {
  path?: Record<string, string>;
  signal?: AbortSignal;
  idempotency_key?: string;
};

export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public request_id?: string,
  ) {
    super(message);
  }
}
export function query_string(query: object = {}) {
  const params = new URLSearchParams();
  for (const [name, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === "") continue;
    for (const item of Array.isArray(value) ? value : [value])
      params.append(name, String(item));
  }
  return params.toString();
}
async function request(
  method: string,
  template: string,
  query: object | undefined,
  body: unknown,
  options: Options,
) {
  const url = template.replace(/\{(\w+)\}/g, (_, key: string) => {
    const value = options.path?.[key];
    if (!value) throw new Error(`Missing path parameter ${key}`);
    return encodeURIComponent(value);
  });
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (options.idempotency_key)
    headers["Idempotency-Key"] = options.idempotency_key;
  const result = await fetch(
    url + (query_string(query) ? `?${query_string(query)}` : ""),
    {
      method,
      headers,
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: options.signal,
    },
  );
  let envelope: DTO<"Envelope">;
  try {
    envelope = (await result.json()) as DTO<"Envelope">;
  } catch {
    throw new ApiError(
      "invalid_response",
      "发生意外错误，操作未完成。请稍后重试。",
    );
  }
  if (!result.ok || envelope.error) {
    const safe = result.status < 500 && envelope.error?.message;
    throw new ApiError(
      envelope.error?.code ?? "unexpected_error",
      safe || "发生意外错误，操作未完成。请稍后重试。",
      envelope.request_id,
    );
  }
  if (
    envelope.status !== result.status ||
    envelope.data === null ||
    envelope.error !== null
  )
    throw new ApiError(
      "invalid_response",
      "发生意外错误，操作未完成。请稍后重试。",
    );
  return envelope.data;
}
export const api = {
  async get<P extends GetPath>(
    path: P,
    query?: Query<Operation<P, "get">>,
    options: Options = {},
  ): Promise<Payload<Operation<P, "get">>> {
    return (await request(
      "GET",
      path,
      query as object | undefined,
      undefined,
      options,
    )) as Payload<Operation<P, "get">>;
  },
  async post<P extends PostPath>(
    path: P,
    body: Body<Operation<P, "post">> | undefined,
    options: Options = {},
  ): Promise<Payload<Operation<P, "post">>> {
    return (await request("POST", path, undefined, body, options)) as Payload<
      Operation<P, "post">
    >;
  },
};

// Keep the key for a failed intent; a confirmed success or changed payload starts a new intent.
export function create_idempotent_post() {
  let pending: { fingerprint: string; key: string } | undefined;
  return async <P extends PostPath>(
    path: P,
    body: Body<Operation<P, "post">> | undefined,
    options: Omit<Options, "idempotency_key"> = {},
  ): Promise<Payload<Operation<P, "post">>> => {
    const fingerprint = JSON.stringify([path, options.path, body]);
    if (!pending || pending.fingerprint !== fingerprint)
      pending = { fingerprint, key: crypto.randomUUID() };
    const intent = pending;
    const data = await api.post(path, body, {
      ...options,
      idempotency_key: intent.key,
    });
    if (pending === intent) pending = undefined;
    return data;
  };
}
