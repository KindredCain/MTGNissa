import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { action_names } from "../src/shared/display-names";
import { uuid } from "../src/test/fixtures";

const actions = [
  "add_card",
  "remove_card",
  "add_wishlist",
  "remove_wishlist",
  "create_deck",
  "delete_deck",
  "copy_deck",
  "modify_deck",
] as const;

test("summary rows open details from every cell, blank space and keyboard", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/operation_logs");
  const row = page.locator("main tbody tr").first();
  const modal = page.getByRole("dialog");
  for (const cell of [0, 1, 2, 3]) {
    const target = row.locator("td").nth(cell);
    const box = (await target.boundingBox())!;
    await target.click({ position: { x: box.width - 8, y: box.height / 2 } });
    await expect(modal).toContainText("历史卡牌名称");
    await expect(page).toHaveURL("/operation_logs");
    await modal.getByRole("button", { name: "×", exact: true }).click();
  }
  const type = row.getByRole("button", { name: "增加卡牌", exact: true });
  for (const key of ["Enter", "Space"]) {
    await type.focus();
    await type.press(key);
    await expect(modal).toContainText("历史卡牌名称");
    await modal.getByRole("button", { name: "×", exact: true }).click();
  }
});

for (const action of actions) {
  test(`${action} tables retain their layout and colors across detail pages`, async ({
    page,
  }, testInfo) => {
    await mock_api(page);
    await page.route("**/api/v1/operation_logs/summary?**", (route) =>
      route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "log-test",
          data: {
            items: actions.map((action_type) => ({
              operation_date: "2026-10-09",
              action_type,
              count: 11,
            })),
            page: 1,
            page_size: 10,
            total_items: 8,
            total_pages: 1,
          },
        },
      }),
    );
    const card_log = actions.indexOf(action) < 4;
    const quantity_log = actions.indexOf(action) < 2;
    const name = "历史名称".repeat(30);
    await page.route("**/api/v1/operation_logs?**", (route) => {
      const params = new URL(route.request().url()).searchParams;
      expect(params.get("operation_date")).toBe("2026-10-09");
      expect(params.get("action_type")).toBe(action);
      const current = Number(params.get("page"));
      return route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "log-test",
          data: {
            items: Array.from({ length: current === 1 ? 10 : 1 }, (_, i) => ({
              operation_id: String((current - 1) * 10 + i + 1),
              action_type: action,
              occurred_at: "2026-10-09T04:00:00Z",
              ...(card_log
                ? {
                    kind: "card",
                    card: {
                      printing_uuid: uuid(1),
                      name,
                      set_code: "KLD",
                      collector_number: "1",
                      language: "en",
                      finish: quantity_log ? "normal" : null,
                      quantity_delta: quantity_log
                        ? action === "add_card"
                          ? 1
                          : -1
                        : null,
                    },
                  }
                : { kind: "deck", deck: { deck_id: "1", name } }),
            })),
            page: current,
            page_size: 10,
            total_items: 11,
            total_pages: 2,
          },
        },
      });
    });
    await page.goto("/operation_logs");
    const summary = page.locator("main table");
    await expect(summary.locator("tbody tr")).toHaveCount(8);
    await expect(summary.locator("th").first()).toHaveCSS(
      "background-color",
      "rgb(28, 42, 35)",
    );
    for (const row of await summary.locator("tbody tr").all()) {
      await expect(row).toHaveCSS("background-color", "rgb(255, 255, 255)");
      expect((await row.boundingBox())!.height).toBe(52);
    }
    await expect(summary.locator("th").nth(2)).toHaveCSS("text-align", "right");
    await summary
      .getByRole("button", { name: action_names[action], exact: true })
      .click();
    const modal = page.getByRole("dialog");
    const rows = modal.locator("tbody tr");
    await expect(rows).toHaveCount(10);
    await expect(modal.locator("th")).toHaveCount(
      quantity_log ? 4 : card_log ? 3 : 2,
    );
    await expect(modal.locator("th").first()).toHaveCSS(
      "background-color",
      "rgb(28, 42, 35)",
    );
    for (const row of await rows.all()) {
      expect((await row.boundingBox())!.height).toBe(46);
      await expect(row).toHaveCSS("background-color", "rgb(255, 255, 255)");
    }
    await expect(rows.first().locator("td").nth(1)).toHaveAttribute(
      "title",
      name,
    );
    if (quantity_log) {
      await expect(rows.first().locator("td").last()).toHaveCSS(
        "text-align",
        "right",
      );
      await expect(rows.first().locator("td").last()).toHaveText(
        action === "add_card" ? "+1" : "-1",
      );
    }
    const before = (await modal.boundingBox())!;
    expect(before.width).toBe(1040);
    expect(before.height).toBe(650);
    const footer = modal.locator("[data-pagination]");
    const footer_top = (await footer.boundingBox())!.y;
    if (action === "add_card") {
      await page.screenshot({ path: testInfo.outputPath("detail.png") });
    }
    await modal.getByRole("button", { name: "下一页", exact: true }).click();
    await expect(rows).toHaveCount(1);
    expect((await modal.boundingBox())!.height).toBe(before.height);
    expect((await footer.boundingBox())!.y).toBe(footer_top);
    await modal.getByRole("button", { name: "×", exact: true }).click();
    await expect(summary.locator("tbody tr")).toHaveCount(8);
  });
}
