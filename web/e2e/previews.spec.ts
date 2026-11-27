import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, context } from "../src/test/fixtures";
import { context_card, with_summary } from "../src/decks/model";

for (const route of ["/decks/1", "/decks/1/edit"]) {
  test(`card description follows page scrolling at ${route}`, async ({
    page,
  }) => {
    await mock_api(page);
    await page.goto(route);
    const target = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await expect(target).toBeVisible();
    if (route.endsWith("/edit"))
      await page.locator("body").evaluate((element) => {
        element.scrollTop = 200;
      });
    await target.hover({ position: { x: 70, y: 100 } });
    const preview = page.getByRole("tooltip");
    await expect(preview).toBeVisible();
    const before = (await preview.boundingBox())!;
    const anchor = (await target.boundingBox())!;
    const opens_left = (await preview.getAttribute("data-side")) === "left";
    const gap = opens_left
      ? anchor.x - before.x - before.width
      : before.x - anchor.x - anchor.width;
    expect(gap).toBeGreaterThanOrEqual(4);
    expect(gap).toBeLessThanOrEqual(7);
    await page.locator("body").evaluate((element) => {
      element.scrollTop += 40;
    });
    await expect
      .poll(async () => (await preview.boundingBox())?.y)
      .toBeCloseTo(before.y - 40, 0);
    await page.mouse.move(1, 1);
    await expect(preview).toHaveCount(0);
  });

  for (const kind of ["shortage", "deck_usage"] as const) {
    test(`${kind} hint and arrow follow page scrolling at ${route}`, async ({
      page,
    }) => {
      const state = await mock_api(page);
      const current = state.deck.sections[1].cards[0];
      current.owned_quantity = kind === "shortage" ? 0 : 2;
      current.other_decks_quantity = kind === "shortage" ? 0 : 3;
      current.other_decks_count = kind === "shortage" ? 0 : 1;
      state.deck = with_summary(state.deck);
      await page.goto(route);
      const badge = page.getByRole("button", {
        name: kind === "shortage" ? "数量不足" : "卡组占用",
        exact: true,
      });
      await expect(badge).toBeVisible();
      // Keep the anchor away from viewport clamping while measuring movement.
      if (route.endsWith("/edit"))
        await page.locator("body").evaluate((element) => {
          element.scrollTop = 120;
        });
      await badge.focus();
      const hint = page.locator("[data-exception-side]");
      await expect(hint).toBeVisible();
      const before = (await hint.boundingBox())!;
      await page.locator("body").evaluate((element) => {
        element.scrollTop += 40;
      });
      await expect
        .poll(async () => (await hint.boundingBox())?.y)
        .toBeCloseTo(before.y - 40, 0);
      const tab = (await badge.boundingBox())!,
        arrow = (await hint.locator("svg").first().boundingBox())!;
      expect(
        Math.abs(arrow.y + arrow.height / 2 - tab.y - tab.height / 2),
      ).toBeLessThanOrEqual(1);
    });
  }

  test(`card description follows section horizontal scrolling at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1].cards.push(
      context_card(context(2, { type_group: "other" }), 1),
    );
    await page.goto(route);
    const target = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await target.focus();
    const preview = page.getByRole("tooltip");
    await expect(preview).toBeVisible();
    const before = (await preview.boundingBox())!;
    await target
      .locator("xpath=ancestor::section[1]")
      .locator('[class*="regionScroll"]')
      .evaluate((element) => {
        element.scrollLeft = 100;
      });
    await expect
      .poll(async () => (await preview.boundingBox())?.x)
      .toBeCloseTo(before.x - 100, 0);
  });

  test(`focused exception follows layout changes above its card at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1].cards.push(
      context_card(context(2, { owned_quantity: 0 }), 1),
    );
    state.deck = with_summary(state.deck);
    await page.goto(route);
    const first = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await first.hover({ position: { x: 70, y: 10 } });
    await expect(first.locator("..")).toHaveCSS("height", "210px");
    const badge = page.getByRole("button", { name: "数量不足", exact: true });
    await badge.focus();
    const hint = page.locator("[data-exception-side]");
    await expect(hint).toBeVisible();
    const before = (await hint.boundingBox())!;
    await page.mouse.move(1, 1);
    await expect(first.locator("..")).toHaveCSS("height", "30px");
    await expect
      .poll(async () => (await hint.boundingBox())?.y)
      .toBeCloseTo(before.y - 180, 0);
  });
}

test("multi-digit deck quantities fit inside the corner marker", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.deck.sections[1].cards[0].quantity = 1234;
  await page.goto("/decks/1");
  const marker = page
    .getByRole("button", { name: "测试卡牌 1，1234 张", exact: true })
    .locator('[class*="quantity"]');
  await expect(marker).toBeVisible();
  const bounds = await marker.evaluate((element) => {
    const range = document.createRange();
    range.selectNodeContents(element);
    const text = range.getBoundingClientRect();
    const box = element.getBoundingClientRect();
    return {
      left: text.left - box.left,
      top: text.top - box.top,
      diagonal: text.right - box.left + text.bottom - box.top,
      size: box.width,
    };
  });
  expect(bounds.left).toBeGreaterThanOrEqual(0);
  expect(bounds.top).toBeGreaterThanOrEqual(0);
  expect(bounds.diagonal).toBeLessThanOrEqual(bounds.size);
});

test("add-card preview follows its focused name while results scroll", async ({
  page,
}) => {
  await mock_api(page);
  await page.route("**/api/v1/decks/1/cards/search**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "preview-test",
        data: {
          items: Array.from({ length: 30 }, (_, index) => ({
            card: card(index + 1),
            faces: context(index + 1).faces,
          })),
          total: 30,
          next_cursor: null,
        },
      },
    }),
  );
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
  await modal.getByRole("button", { name: "搜索", exact: true }).click();
  await modal.getByRole("button", { name: "测试卡牌 5", exact: true }).focus();
  const preview = page.getByRole("tooltip");
  await expect(preview).toBeVisible();
  const before = (await preview.boundingBox())!;
  await modal.locator('[class*="results"]').evaluate((element) => {
    element.scrollTop += 40;
  });
  await expect
    .poll(async () => (await preview.boundingBox())?.y)
    .toBeCloseTo(before.y - 40, 0);
});
