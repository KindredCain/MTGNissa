import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { api, ApiError, create_idempotent_post, query_string } from "./client";
import { EditingSession, get_contexts } from "../decks/contexts";
import { QueryClient } from "@tanstack/react-query";
import { uuid } from "../test/fixtures";
const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
describe("API boundary", () => {
  it("reuses a failed creation intent after a lost response and starts a new one after success", async () => {
    const completed = new Map<string, string>();
    const keys: string[] = [];
    let attempts = 0;
    server.use(
      http.post("/api/v1/decks", async ({ request }) => {
        const key = request.headers.get("Idempotency-Key")!;
        const body = (await request.json()) as { name: string };
        keys.push(key);
        const replay = completed.has(key);
        if (!replay) completed.set(key, String(completed.size + 1));
        if (++attempts === 1) return HttpResponse.error();
        const status = replay ? 200 : 201;
        return HttpResponse.json(
          {
            status,
            data: { deck_id: completed.get(key), name: body.name },
            error: null,
            request_id: "test",
          },
          { status },
        );
      }),
    );
    const post = create_idempotent_post();
    await expect(post("/api/v1/decks", { name: "Test" })).rejects.toThrow();
    expect((await post("/api/v1/decks", { name: "Test" })).deck_id).toBe("1");
    expect(completed.size).toBe(1);
    expect((await post("/api/v1/decks", { name: "Test" })).deck_id).toBe("2");
    expect(keys[0]).toBe(keys[1]);
    expect(keys[2]).not.toBe(keys[1]);
  });

  it("starts a new intent when a failed request's payload or resource changes", async () => {
    const keys: string[] = [];
    server.use(
      http.post("/api/v1/cards/:id/collection_adjustments", ({ request }) => {
        keys.push(request.headers.get("Idempotency-Key")!);
        return HttpResponse.error();
      }),
    );
    const post = create_idempotent_post();
    for (const [id, delta] of [
      [1, 1],
      [1, -1],
      [2, -1],
    ] as const) {
      await expect(
        post(
          "/api/v1/cards/{printing_uuid}/collection_adjustments",
          { finish: "normal", delta },
          { path: { printing_uuid: uuid(id) } },
        ),
      ).rejects.toThrow();
    }
    expect(new Set(keys).size).toBe(3);
  });

  it("serializes repeated array parameters without comma joining", () => {
    expect(
      query_string({
        language: ["en", "ja"],
        q: 'otag:"mana rock"',
        merge_sets: false,
      }),
    ).toBe("language=en&language=ja&q=otag%3A%22mana+rock%22&merge_sets=false");
  });
  it("preserves the business key and sends idempotency only for the intended mutation", async () => {
    server.use(
      http.post(
        "/api/v1/cards/:id/wishlist_state",
        async ({ request, params }) => {
          expect(params.id).toBe(uuid(1));
          expect(request.headers.get("Idempotency-Key")).toBe(uuid(9));
          expect(await request.json()).toEqual({ included: true });
          return HttpResponse.json({
            status: 200,
            data: { printing_uuid: uuid(1), oracle_id: null, included: true },
            error: null,
            request_id: "test",
          });
        },
      ),
    );
    expect(
      (
        await api.post(
          "/api/v1/cards/{printing_uuid}/wishlist_state",
          { included: true },
          { path: { printing_uuid: uuid(1) }, idempotency_key: uuid(9) },
        )
      ).included,
    ).toBe(true);
  });
  it("does not expose an internal server error to the UI", async () => {
    server.use(
      http.get("/api/v1/app", () =>
        HttpResponse.json(
          {
            status: 500,
            data: null,
            error: { code: "internal_error", message: "SQL internal path" },
            request_id: "x",
          },
          { status: 500 },
        ),
      ),
    );
    await expect(api.get("/api/v1/app")).rejects.toMatchObject({
      message: "发生意外错误，操作未完成。请稍后重试。",
    });
  });
  it("refuses incomplete card contexts before editor state can be replaced", async () => {
    const session = new EditingSession();
    session.activate();
    server.use(
      http.post("/api/v1/decks/1/card_contexts", () =>
        HttpResponse.json({
          status: 200,
          data: { items: [] },
          error: null,
          request_id: "x",
        }),
      ),
    );
    await expect(
      get_contexts(new QueryClient(), "1", session, [uuid(1)]),
    ).rejects.toBeInstanceOf(ApiError);
  });
});
