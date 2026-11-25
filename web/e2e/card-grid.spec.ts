import { expect, test } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, uuid } from "../src/test/fixtures";

for (const route of [
  "/sets/KLD/cards?language=en",
  "/sets/TKLD/tokens",
  "/search?q=test",
  "/collection?merge_languages=false",
  "/wishlist",
]) {
  test(`card grids adapt without scaling at ${route}`, async ({ page }) => {
    await mock_api(page);
    const cards = Array.from({ length: 42 }, (_, index) => card(index + 1));
    await page.route(
      /\/api\/v1\/(cards|tokens|search|collection|wishlist)(?:\?|$)/,
      async (request) => {
        const endpoint = new URL(request.request().url()).pathname
          .split("/")
          .at(-1);
        let data: unknown;
        if (endpoint === "cards") {
          data = {
            set: cards[0]!.set,
            language: cards[0]!.language,
            items: cards,
            total: cards.length,
          };
        } else if (endpoint === "tokens") {
          data = {
            set: cards[0]!.set,
            total: cards.length,
            items: cards.map((item, index) => ({
              token_uuid: uuid(index + 1),
              name: item.name,
              collector_number: String(index + 1),
              layout: "token",
              images: item.images,
            })),
          };
        } else if (endpoint === "search") {
          data = {
            input: "test",
            mode: "syntax",
            executed_query: "test",
            items: cards,
            total: cards.length,
            next_cursor: null,
          };
        } else {
          data = {
            items: cards.map((item) => ({
              card: item,
              group_set: item.set,
              group_summary: { total_quantity: 84, distinct_cards: 42 },
              normal_quantity: 2,
              foil_quantity: 0,
              merged_sets: false,
              merged_languages: false,
              included: true,
            })),
            total: cards.length,
            summary: { total_quantity: 84, distinct_cards: 42 },
            next_cursor: null,
          };
        }
        await request.fulfill({
          json: { status: 200, data, error: null, request_id: "test" },
        });
      },
    );
    await page.goto(route);
    await expect(
      page.locator("[data-card-grid-page] article").first(),
    ).toBeVisible();
    let previous_count = 0;
    for (const [width, columns] of [
      [320, 1],
      [600, 2],
      [601, 2],
      [900, 3],
      [1100, 3],
      [1280, 4],
      [1366, 4],
      [1440, 5],
      [1920, 6],
      [2560, 9],
    ]) {
      await page.setViewportSize({ width: width!, height: 900 });
      await expect
        .poll(() =>
          page
            .locator("[data-card-grid-page] article")
            .first()
            .evaluate((tile) => {
              const grid = tile.parentElement!;
              const first_top = tile.getBoundingClientRect().top;
              return [...grid.children].filter(
                (item) =>
                  Math.abs(item.getBoundingClientRect().top - first_top) < 1,
              ).length;
            }),
        )
        .toBe(columns);
      const geometry = await page
        .locator("[data-card-grid-page] article")
        .first()
        .evaluate((tile) => ({
          card_width: tile.getBoundingClientRect().width,
          image_height: tile.firstElementChild!.getBoundingClientRect().height,
          gap: getComputedStyle(tile.parentElement!).columnGap,
          page_width: document.body.scrollWidth,
          viewport_width: document.body.clientWidth,
        }));
      expect(geometry.card_width).toBe(244);
      expect(geometry.image_height).toBe(340);
      expect(geometry.gap).toBe("35px");
      expect(geometry.page_width).toBe(geometry.viewport_width);
      if (route.startsWith("/sets/")) {
        const tiles = page.locator("[data-card-grid-page] article");
        await expect
          .poll(async () => {
            const count = await tiles.count();
            return count === cards.length ? 0 : count % columns!;
          })
          .toBe(0);
        const count = await tiles.count();
        expect(count).toBeGreaterThanOrEqual(previous_count);
        previous_count = count;
      }
      if ([320, 900, 2560].includes(width!)) {
        await page.screenshot({
          path: `test-results/${test.info().project.name}${route.split("?")[0]!.replaceAll("/", "-")}-${width}-grid.png`,
        });
      }
    }
  });
}
