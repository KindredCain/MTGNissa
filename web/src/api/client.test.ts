import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { api, ApiError, query_string } from "./client";
import { get_contexts } from "../decks/contexts";
import { QueryClient } from "@tanstack/react-query";
import { uuid } from "../test/fixtures";
const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
describe("API boundary", () => {
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
      get_contexts(new QueryClient(), "1", [uuid(1)]),
    ).rejects.toBeInstanceOf(ApiError);
  });
});
