import { test, expect, type Page } from "@playwright/test";
import type { Query, QueryCacheNotifyEvent } from "@tanstack/react-query";
import { mock_api } from "./mock-api";
import { card, deck, detail, uuid } from "../src/test/fixtures";

type DeleteEffects = {
  detail_removed: boolean;
  decks_invalidated: boolean;
  logs_invalidated: boolean;
};
type DeleteWindow = typeof window & { delete_effects?: DeleteEffects };
type InvalidationWindow = typeof window & { invalidated_queries?: string[] };

async function scroll_body(page: Page, top: number) {
  const expected = await page.evaluate((value) => {
    document.body.scrollTo({ top: value, behavior: "instant" });
    return document.body.scrollTop;
  }, top);
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(expected);
  return expected;
}

async function mock_details(page: Page) {
  const initial = detail();
  initial.faces[0]!.display_oracle_text = "卡牌规则说明。\n".repeat(60);
  const language = {
    ...initial.languages[0]!,
    printing_uuid: uuid(2),
    language: { code: "ja", sort_order: 2 },
  };
  initial.languages.push(language);
  initial.other_versions = [
    {
      set: { ...initial.card.set, code: "TST", display_name: "其他系列" },
      collector_number: "2",
      released_at: initial.card.released_at,
      normal_quantity: 0,
      foil_quantity: 0,
      languages: [language],
    },
  ];
  for (const id of [uuid(1), uuid(2)])
    await page.route(`**/api/v1/cards/${id}`, (route) =>
      route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "test",
          data: { ...initial, card: { ...initial.card, printing_uuid: id } },
        },
      }),
    );
}

for (const action of ["create", "copy-list", "copy-detail"] as const) {
  for (const destination of ["stay", "leave", "return"] as const) {
    test(`pending ${action} respects ${destination} and preserves successful cache invalidation`, async ({
      page,
    }) => {
      await mock_api(page);
      const origin = action === "copy-detail" ? "/decks/1" : "/decks";
      const endpoint =
        action === "create" ? "/api/v1/decks" : "/api/v1/decks/1/copies";
      const created = {
        ...deck(),
        deck_id: "2",
        name: action === "create" ? "新建卡组" : "复制 测试卡组",
      };
      await page.route("**/api/v1/decks/2", (route) =>
        route.fulfill({
          json: { status: 200, data: created, error: null, request_id: "test" },
        }),
      );
      let release!: () => void;
      const pending = new Promise<void>((resolve) => {
        release = resolve;
      });
      await page.route(`**${endpoint}`, async (route) => {
        if (route.request().method() !== "POST") return route.fallback();
        await pending;
        await route.fulfill({
          status: 201,
          json: {
            status: 201,
            data: { deck_id: created.deck_id, name: created.name },
            error: null,
            request_id: "test",
          },
        });
      });
      let release_logs!: () => void;
      if (destination === "leave") {
        const pending_logs = new Promise<void>((resolve) => {
          release_logs = resolve;
        });
        await page.route(
          "**/api/v1/operation_logs/summary?**",
          async (route) => {
            await pending_logs;
            await route.fallback();
          },
        );
      }
      await page.goto(origin);
      await expect(
        page.getByRole("button", {
          name:
            action === "copy-list"
              ? "更多卡组操作"
              : action === "create"
                ? "新建卡组"
                : "复制卡组",
          exact: true,
        }),
      ).toBeVisible();
      await page.evaluate(async () => {
        const query_path = "/src/api/query.ts";
        const list_path = "/src/api/lists.ts";
        const { query_client } = await import(/* @vite-ignore */ query_path);
        const { decks_options } = await import(/* @vite-ignore */ list_path);
        await query_client.fetchInfiniteQuery(
          decks_options(new URLSearchParams()),
        );
        const invalidated: string[] = [];
        (window as InvalidationWindow).invalidated_queries = invalidated;
        query_client
          .getQueryCache()
          .subscribe((event: QueryCacheNotifyEvent) => {
            if (event.type === "updated" && event.action.type === "invalidate")
              invalidated.push(event.query.queryHash);
          });
      });
      if (action === "copy-list")
        await page.getByRole("button", { name: "更多卡组操作" }).click();
      const requested = page.waitForRequest(
        (request) =>
          new URL(request.url()).pathname === endpoint &&
          request.method() === "POST",
      );
      await page
        .getByRole("button", {
          name: action === "create" ? "新建卡组" : "复制卡组",
          exact: true,
        })
        .click();
      await requested;
      if (destination !== "stay") {
        await page
          .getByRole("navigation", { name: "主导航" })
          .getByRole("link", { name: "操作日志", exact: true })
          .click();
        await expect(page).toHaveURL("/operation_logs");
        await expect(
          page.getByRole("heading", { name: "操作日志", exact: true }),
        ).toBeVisible();
        if (destination === "return") {
          await page.goBack();
          await expect(page).toHaveURL(origin);
        }
      } else if (action === "create") {
        // Filtering the current page must not count as leaving it.
        await page.getByRole("combobox", { name: "卡组排序" }).click();
        await page.getByRole("option", { name: "名称 ↑", exact: true }).click();
        await expect(page).toHaveURL("/decks?sort=name_asc");
      }
      const targets = await page.evaluate(async () => {
        const path = "/src/api/query.ts";
        const { query_client } = await import(/* @vite-ignore */ path);
        return {
          lists: query_client
            .getQueryCache()
            .findAll({ queryKey: ["decks", "list"] })
            .map((query: Query) => query.queryHash),
          logs: query_client
            .getQueryCache()
            .findAll({ queryKey: ["operation_logs"] })
            .map((query: Query) => query.queryHash),
        };
      });
      expect(targets.lists.length).toBeGreaterThan(0);
      if (destination !== "stay")
        expect(targets.logs.length).toBeGreaterThan(0);
      const expected = [
        ...targets.lists,
        ...(destination === "stay" ? [] : targets.logs),
      ];
      release();
      await expect
        .poll(() =>
          page.evaluate((hashes) => {
            const invalidated =
              (window as InvalidationWindow).invalidated_queries ?? [];
            return hashes.every((hash) => invalidated.includes(hash));
          }, expected),
        )
        .toBe(true);
      if (destination === "leave") {
        // A fetch finishing after invalidation may legitimately mark the cache fresh.
        release_logs();
        await expect(page.locator("tbody tr")).toHaveCount(1);
      }
      if (destination === "stay") {
        await expect(page).toHaveURL("/decks/2/edit");
        await expect(
          page.getByRole("textbox", { name: "卡组名称", exact: true }),
        ).toHaveValue(created.name);
      } else {
        await expect(page).toHaveURL(
          destination === "leave" ? "/operation_logs" : origin,
        );
        expect(
          await page
            .getByRole("textbox", { name: "卡组名称", exact: true })
            .count(),
        ).toBe(0);
      }
    });
  }
}

for (const origin of ["list", "detail"] as const) {
  for (const outcome of ["stay", "leave", "return", "failure"] as const) {
    test(`pending delete from ${origin} respects ${outcome}`, async ({
      page,
    }) => {
      await mock_api(page);
      let release!: () => void;
      const pending = new Promise<void>((resolve) => {
        release = resolve;
      });
      await page.route("**/api/v1/decks/1/delete", async (route) => {
        await pending;
        await route.fulfill(
          outcome === "failure"
            ? {
                status: 503,
                json: {
                  status: 503,
                  data: null,
                  error: {
                    code: "dependency_unavailable",
                    message: "unavailable",
                  },
                  request_id: "test",
                },
              }
            : {
                json: {
                  status: 200,
                  data: { deck_id: "1", deleted: true },
                  error: null,
                  request_id: "test",
                },
              },
        );
      });
      await page.goto("/sets");
      async function enter_origin() {
        await page
          .getByRole("navigation", { name: "主导航" })
          .getByRole("link", { name: "卡组", exact: true })
          .click();
        await expect(
          page.getByRole("link", { name: "查看测试卡组", exact: true }),
        ).toBeVisible();
        if (origin === "detail")
          await page
            .getByRole("link", { name: "查看测试卡组", exact: true })
            .click();
        await expect(
          page.getByRole("button", {
            name: origin === "list" ? "更多卡组操作" : "删除卡组",
            exact: true,
          }),
        ).toBeVisible();
      }
      await enter_origin();
      await page.evaluate(async () => {
        const query_path = "/src/api/query.ts";
        const client_path = "/src/api/client.ts";
        const { query_client } = await import(/* @vite-ignore */ query_path);
        const { api } = await import(/* @vite-ignore */ client_path);
        await query_client.fetchQuery({
          queryKey: ["decks", "detail", "1"],
          queryFn: () =>
            api.get("/api/v1/decks/{deck_id}", undefined, {
              path: { deck_id: "1" },
            }),
        });
        await query_client.fetchQuery({
          queryKey: ["operation_logs", "summary", { page: 1 }],
          queryFn: () => api.get("/api/v1/operation_logs/summary", { page: 1 }),
        });
        const effects: DeleteEffects = {
          detail_removed: false,
          decks_invalidated: false,
          logs_invalidated: false,
        };
        (window as DeleteWindow).delete_effects = effects;
        query_client
          .getQueryCache()
          .subscribe((event: QueryCacheNotifyEvent) => {
            const key = event.query.queryKey;
            if (
              event.type === "removed" &&
              key[0] === "decks" &&
              key[1] === "detail" &&
              key[2] === "1"
            )
              effects.detail_removed = true;
            if (
              event.type === "updated" &&
              event.action.type === "invalidate"
            ) {
              if (key[0] === "decks") effects.decks_invalidated = true;
              if (key[0] === "operation_logs") effects.logs_invalidated = true;
            }
          });
      });
      if (origin === "list")
        await page
          .getByRole("button", { name: "更多卡组操作", exact: true })
          .click();
      await page.getByRole("button", { name: "删除卡组", exact: true }).click();
      const requested = page.waitForRequest((request) =>
        request.url().endsWith("/api/v1/decks/1/delete"),
      );
      await page
        .getByRole("dialog", { name: "删除卡组？", exact: true })
        .getByRole("button", { name: "删除卡组", exact: true })
        .click();
      await requested;
      if (outcome === "leave" || outcome === "return") {
        await page.goBack();
        await expect(page).toHaveURL(origin === "list" ? "/sets" : "/decks");
        if (origin === "detail") {
          await page
            .getByRole("navigation", { name: "主导航" })
            .getByRole("link", { name: "卡牌中心", exact: true })
            .click();
          await expect(page).toHaveURL("/sets");
        }
        await expect(
          page.getByRole("heading", { name: "全部卡牌", exact: true }),
        ).toBeVisible();
        if (outcome === "return") await enter_origin();
      }
      release();
      if (outcome === "failure") {
        const error = page.getByRole("dialog", {
          name: "操作失败",
          exact: true,
        });
        await expect(error).toBeVisible();
        await error.getByRole("button", { name: "关闭", exact: true }).click();
        expect(
          await page.evaluate(() => (window as DeleteWindow).delete_effects),
        ).toEqual({
          detail_removed: false,
          decks_invalidated: false,
          logs_invalidated: false,
        });
      } else {
        await expect
          .poll(() =>
            page.evaluate(() => (window as DeleteWindow).delete_effects),
          )
          .toEqual({
            detail_removed: true,
            decks_invalidated: true,
            logs_invalidated: true,
          });
      }
      await expect(page).toHaveURL(
        outcome === "stay"
          ? "/decks"
          : outcome === "leave"
            ? "/sets"
            : origin === "list"
              ? "/decks"
              : "/decks/1",
      );
      if (outcome === "failure")
        await expect(
          page.getByRole(origin === "list" ? "link" : "heading", {
            name: origin === "list" ? "查看测试卡组" : "测试卡组",
            exact: true,
          }),
        ).toBeVisible();
    });
  }
}

test("cached detail navigation starts at top and history restores each body position", async ({
  page,
}) => {
  await mock_api(page);
  await mock_details(page);
  await page.goto("/sets/KLD/cards?language=en");
  const link = page.getByRole("link", { name: "查看测试卡牌 1", exact: true });
  await link.click();
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await expect(
    page.getByRole("heading", { name: "规则文字", exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(link).toBeVisible();
  const list_position = await scroll_body(page, 700);
  expect(list_position).toBe(700);
  await link.evaluate((element: HTMLAnchorElement) => element.click());
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await expect(
    page.getByRole("heading", { name: "规则文字", exact: true }),
  ).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  const detail_position = await scroll_body(page, 450);
  expect(detail_position).toBe(450);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(list_position);
  await page.goForward();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(detail_position);
});

test("history restoration waits for a refetched and locally expanded list", async ({
  page,
}) => {
  await mock_api(page);
  await mock_details(page);
  await page.goto("/sets/KLD/cards?language=en");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  await scroll_body(page, 100_000);
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 42", exact: true }),
  ).toHaveCount(1);
  const position = await scroll_body(page, 100_000);
  expect(position).toBeGreaterThan(2000);
  await page
    .getByRole("link", { name: "查看测试卡牌 1", exact: true })
    .evaluate((element: HTMLAnchorElement) => element.click());
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await page.evaluate(async () => {
    const path = "/src/api/query.ts";
    const { query_client } = await import(/* @vite-ignore */ path);
    query_client.removeQueries({ queryKey: ["cards", "set"] });
  });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/cards?**", async (route) => {
    await pending;
    await route.fallback();
  });
  await page.goBack();
  await expect(page.locator("[data-scroll-loading]")).toBeVisible();
  release();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 42", exact: true }),
  ).toHaveCount(1);
});

for (const action of ["language", "version"] as const) {
  test(`card ${action} updates preserve the body scroll position`, async ({
    page,
  }) => {
    await mock_api(page);
    await mock_details(page);
    await page.goto(`/cards/${uuid(1)}`);
    await expect(
      page.getByRole("combobox", { name: "当前系列语言" }),
    ).toBeVisible();
    const position = await scroll_body(page, 500);
    expect(position).toBe(500);
    if (action === "language") {
      await page
        .getByRole("combobox", { name: "当前系列语言" })
        .evaluate((element: HTMLButtonElement) => {
          element.dispatchEvent(
            new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }),
          );
        });
      await page
        .getByRole("option", { name: "日文", exact: true })
        .evaluate((element: HTMLElement) => {
          element.focus({ preventScroll: true });
          element.dispatchEvent(
            new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
          );
        });
    } else
      await page
        .getByRole("link", { name: "日文", exact: true })
        .evaluate((element: HTMLAnchorElement) => element.click());
    await expect(page).toHaveURL(`/cards/${uuid(2)}`);
    await expect
      .poll(() => page.evaluate(() => document.body.scrollTop))
      .toBe(position);
  });
}

test("global search resets the same search page while back restores its scroll", async ({
  page,
}) => {
  await mock_api(page);
  await page.route("**/api/v1/search?**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "test",
        data: {
          input: "test",
          mode: "name",
          executed_query: "test",
          items: Array.from({ length: 42 }, (_, index) => card(index + 1)),
          total: 42,
          next_cursor: null,
        },
      },
    }),
  );
  await page.goto("/search?q=first");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  const position = await scroll_body(page, 700);
  const search = page.getByRole("textbox", { name: "搜索卡牌或输入语法…" });
  await search.fill("second");
  await search.press("Enter");
  await expect(page).toHaveURL("/search?q=second");
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
});

test("list filters still reset to top and history restores the previous list position", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  const position = await scroll_body(page, 700);
  await page.getByRole("combobox", { name: "排序", exact: true }).click();
  await page.getByRole("option", { name: "名称 ↓", exact: true }).click();
  await expect(page).toHaveURL(/sort=name_desc/);
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
});
