import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, context } from "../src/test/fixtures";

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

test("local Set rendering continues beyond two batches without another request", async ({
  page,
}) => {
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
  await expect(items).toHaveCount(30);
  const initial_requests = requests;
  for (const count of [60, 90, 95]) {
    await page.locator("body").evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await expect(items).toHaveCount(count);
  }
  expect(requests).toBe(initial_requests);
});
