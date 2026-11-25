import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, context } from "../src/test/fixtures";

test("page pagination starts before the last card row enters the viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 2560, height: 900 });
  await mock_api(page);
  let requests = 0;
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/search?**", async (route) => {
    const next = new URL(route.request().url()).searchParams.has("cursor");
    if (next) {
      requests++;
      await pending;
    }
    await route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "prefetch-test",
        data: {
          input: "test",
          mode: "name",
          executed_query: "test",
          items: next
            ? [card(31)]
            : Array.from({ length: 30 }, (_, index) => card(index + 1)),
          total: 31,
          next_cursor: next ? null : "next",
        },
      },
    });
  });
  await page.goto("/search?q=test");
  const items = page.locator("article");
  await expect(items).toHaveCount(30);
  expect(requests).toBe(0);
  try {
    await page.locator("body").evaluate((body) => {
      const sentinel = document.querySelector('[class*="loadMore"]')!;
      body.scrollTop +=
        sentinel.getBoundingClientRect().top - body.clientHeight - 360;
    });
    await expect.poll(() => requests).toBe(1);
    expect(
      await items
        .last()
        .evaluate(
          (item) =>
            item.getBoundingClientRect().top > document.body.clientHeight,
        ),
    ).toBe(true);
    await page.mouse.wheel(0, 100);
    await page.mouse.wheel(0, 100);
    expect(requests).toBe(1);
  } finally {
    release();
  }
  await expect(items).toHaveCount(31);
});

for (const kind of ["page", "modal"] as const) {
  test(`${kind} pagination waits for deliberate scrolling after an error and retries the same cursor`, async ({
    page,
  }) => {
    await mock_api(page);
    const first_count = kind === "page" ? 30 : 20;
    const requests: string[] = [];
    let release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    const endpoint =
      kind === "page"
        ? "**/api/v1/search?**"
        : "**/api/v1/decks/1/cards/search**";
    await page.route(endpoint, async (route) => {
      const cursor = new URL(route.request().url()).searchParams.get("cursor");
      if (cursor) requests.push(cursor);
      if (cursor === "next" && requests.length <= 2) {
        await route.fulfill({
          status: 503,
          json: {
            status: 503,
            data: null,
            error: { code: "dependency_unavailable", message: "unavailable" },
            request_id: "pagination-test",
          },
        });
        return;
      }
      if (cursor === "next") await pending;
      const cards = cursor
        ? [card(first_count + (cursor === "next" ? 1 : 2))]
        : Array.from({ length: first_count }, (_, index) => card(index + 1));
      await route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "pagination-test",
          data: {
            ...(kind === "page"
              ? { input: "test", mode: "name", executed_query: "test" }
              : {}),
            items:
              kind === "page"
                ? cards
                : cards.map((item) => ({ card: item, faces: context().faces })),
            total: first_count + 2,
            next_cursor: cursor === "last" ? null : cursor ? "last" : "next",
          },
        },
      });
    });
    if (kind === "page") await page.goto("/search?q=test");
    else {
      await page.goto("/decks/1/edit");
      await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "搜索", exact: true })
        .click();
    }
    const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
    const root =
      kind === "page"
        ? page.locator("body")
        : modal.locator('[class*="results"]');
    const wheel_position = {
      x: kind === "page" ? 5 : (await root.boundingBox())!.width - 10,
      y: 5,
    };
    const items =
      kind === "page"
        ? page.getByRole("link", { name: /^查看测试卡牌/ })
        : modal.getByRole("spinbutton");
    const error = page.getByRole("dialog", { name: "操作失败", exact: true });
    await expect(items).toHaveCount(first_count);
    await root.evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await expect(error).toBeVisible();
    expect(requests).toEqual(["next"]);
    await error.hover();
    await page.mouse.wheel(0, 800);
    expect(requests).toEqual(["next"]);
    await error.getByRole("button", { name: "关闭", exact: true }).click();
    // Closing a dialog, restoring scroll, or scrolling upward must not retry.
    await root.evaluate((element) => {
      element.scrollTop = 0;
    });
    await root.evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await root.hover({ position: wheel_position });
    await page.mouse.wheel(0, -100);
    await page.waitForTimeout(200);
    expect(requests).toEqual(["next"]);
    await page.mouse.wheel(0, 500);
    await expect(error).toBeVisible();
    expect(requests).toEqual(["next", "next"]);
    await error.getByRole("button", { name: "关闭", exact: true }).click();
    await page.waitForTimeout(200);
    expect(requests).toEqual(["next", "next"]);
    // Keyboard scrolling can recover too; repeated input cannot duplicate it.
    if (kind === "page") await items.last().focus();
    else
      await modal
        .getByRole("button", { name: `测试卡牌 ${first_count}`, exact: true })
        .focus();
    await page.keyboard.press("End");
    await expect.poll(() => requests.length).toBe(3);
    await root.hover({ position: wheel_position });
    await page.mouse.wheel(0, 500);
    await page.mouse.wheel(0, 500);
    expect(requests).toEqual(["next", "next", "next"]);
    release();
    await expect.poll(() => items.count()).toBeGreaterThan(first_count);
    await root.evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await expect(items).toHaveCount(first_count + 2);
    expect(requests).toEqual(["next", "next", "next", "last"]);
  });
}

for (const kind of ["collection", "wishlist", "decks"] as const) {
  test(`${kind} sorting menu does not retry failed pagination`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    let requests = 0;
    await page.route(`**/api/v1/${kind}?**`, async (route) => {
      if (new URL(route.request().url()).searchParams.has("cursor")) {
        requests++;
        await route.fulfill({
          status: 503,
          json: {
            status: 503,
            data: null,
            error: { code: "dependency_unavailable", message: "unavailable" },
            request_id: "menu-retry-test",
          },
        });
        return;
      }
      const items = Array.from({ length: 30 }, (_, index) => {
        if (kind === "decks")
          return {
            deck_id: String(index + 1),
            name: `测试卡组 ${index + 1}`,
            created_at: state.deck.created_at,
            updated_at: state.deck.updated_at,
            cover_card: card(index + 1),
            section_counts: {
              commander: 0,
              mainboard: 2,
              sideboard: 0,
              maybeboard: 0,
            },
            counted_total: 2,
            color_distribution: { W: 0, U: 0, B: 0, R: 0, G: 2, C: 0 },
            exception: null,
          };
        return {
          card: card(index + 1),
          group_set: card().set,
          group_summary: { total_quantity: 60, distinct_cards: 30 },
          normal_quantity: 2,
          foil_quantity: 0,
          merged_sets: false,
          merged_languages: false,
          included: true,
        };
      });
      await route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "menu-retry-test",
          data: {
            items,
            total: 31,
            next_cursor: "next",
            ...(kind === "collection"
              ? { summary: { total_quantity: 60, distinct_cards: 30 } }
              : {}),
          },
        },
      });
    });
    await page.goto(
      kind === "collection" ? "/collection?merge_languages=false" : `/${kind}`,
    );
    await expect(page.locator("article")).toHaveCount(30);
    await page.locator("body").evaluate((body) => {
      body.scrollTop = body.scrollHeight;
    });
    const error = page.getByRole("dialog", { name: "操作失败", exact: true });
    await expect(error).toBeVisible();
    await error.getByRole("button", { name: "关闭", exact: true }).click();
    await page
      .getByRole("combobox", {
        name: kind === "decks" ? "卡组排序" : "排序",
        exact: true,
      })
      .click();
    const menu = page.getByRole("listbox");
    await expect(menu).toBeVisible();
    await menu.hover();
    await page.mouse.wheel(0, 200);
    await page.waitForTimeout(200);
    expect(requests).toBe(1);
    await expect(menu).toBeVisible();
    await page.keyboard.press("End");
    await page.waitForTimeout(100);
    expect(requests).toBe(1);
    await expect(menu).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);
    expect(requests).toBe(1);
    await page.mouse.move(5, 400);
    await page.mouse.wheel(0, 200);
    await expect(error).toBeVisible();
    expect(requests).toBe(2);
  });
}

for (const input of ["keyboard", "end_at_bottom", "drag", "track"] as const) {
  test(`page scrollbar retries failed pagination with ${input}`, async ({
    page,
  }) => {
    await mock_api(page);
    let requests = 0;
    let release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route("**/api/v1/search?**", async (route) => {
      const next = new URL(route.request().url()).searchParams.has("cursor");
      if (next) {
        requests++;
        if (requests === 1) {
          await route.fulfill({
            status: 503,
            json: {
              status: 503,
              data: null,
              error: { code: "dependency_unavailable", message: "unavailable" },
              request_id: "scrollbar-retry-test",
            },
          });
          return;
        }
        await pending;
      }
      await route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "scrollbar-retry-test",
          data: {
            input: "test",
            mode: "name",
            executed_query: "test",
            items: next
              ? [card(31)]
              : Array.from({ length: 30 }, (_, index) => card(index + 1)),
            total: 31,
            next_cursor: next ? null : "next",
          },
        },
      });
    });
    await page.goto("/search?q=test");
    await expect(page.locator("article")).toHaveCount(30);
    await page.locator("body").evaluate((body) => {
      body.scrollTop = body.scrollHeight;
    });
    const error = page.getByRole("dialog", { name: "操作失败", exact: true });
    await expect(error).toBeVisible();
    await error.getByRole("button", { name: "关闭", exact: true }).click();
    const bar = page.getByRole("scrollbar", {
      name: "页面纵向滚动",
      exact: true,
    });
    await expect(bar).toBeVisible();
    await page.locator("body").evaluate((body) => {
      body.scrollTop = 0;
    });
    await page.locator("body").evaluate((body) => {
      body.scrollTop = body.scrollHeight;
    });
    await page.waitForTimeout(200);
    expect(requests).toBe(1);
    await bar.focus();
    await bar.press("ArrowUp");
    await page.waitForTimeout(100);
    expect(requests).toBe(1);
    await bar.press("Home");
    await expect
      .poll(() => page.evaluate(() => document.body.scrollTop))
      .toBe(0);
    await bar.press("ArrowDown");
    await page.waitForTimeout(100);
    expect(requests).toBe(1);
    await page.locator("body").evaluate((body) => {
      body.scrollTop = body.scrollHeight;
    });
    await page.waitForTimeout(200);
    expect(requests).toBe(1);
    try {
      if (input === "end_at_bottom") {
        await page.locator("body").evaluate((body) => {
          body.scrollTop = body.scrollHeight;
        });
        await expect
          .poll(() =>
            bar.evaluate(
              (element: HTMLInputElement) => element.value === element.max,
            ),
          )
          .toBe(true);
        await bar.press("End");
      } else {
        await bar.press("Home");
        await expect
          .poll(() => page.evaluate(() => document.body.scrollTop))
          .toBe(0);
        expect(requests).toBe(1);
        if (input === "keyboard") await bar.press("End");
        else {
          const box = (await bar.boundingBox())!;
          if (input === "track")
            await bar.click({
              position: { x: box.width / 2, y: box.height - 2 },
            });
          else {
            const thumb = await bar.evaluate((element) =>
              parseFloat(
                getComputedStyle(element).getPropertyValue("--thumb-size"),
              ),
            );
            await page.mouse.move(box.x + box.width / 2, box.y + thumb / 2);
            await page.mouse.down();
            await page.mouse.move(
              box.x + box.width / 2,
              box.y + box.height - 2,
              { steps: 8 },
            );
            await page.mouse.up();
          }
        }
      }
      await expect.poll(() => requests).toBe(2);
      await bar.press("End");
      await bar.press("PageDown");
      expect(requests).toBe(2);
    } finally {
      release();
    }
    await expect(page.locator("article")).toHaveCount(31);
  });
}

test("local Set rendering continues beyond two batches without another request", async ({
  page,
}, info) => {
  await mock_api(page);
  let requests = 0;
  await page.route("**/api/v1/cards?**", async (route) => {
    requests++;
    await route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "pagination-test",
        data: {
          set: card().set,
          language: card().language,
          items: Array.from({ length: 95 }, (_, index) => card(index + 1)),
          total: 95,
        },
      },
    });
  });
  await page.goto("/sets/KLD/cards?language=en");
  const items = page.getByRole("link", { name: /^查看测试卡牌/ });
  const batch = info.project.name === "desktop-1440" ? 30 : 32;
  await expect(items).toHaveCount(batch);
  const initial_requests = requests;
  for (
    let count = Math.min(batch * 2, 95);
    ;
    count = Math.min(count + batch, 95)
  ) {
    await page.locator("body").evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await expect(items).toHaveCount(count);
    if (count === 95) break;
  }
  expect(requests).toBe(initial_requests);
});
