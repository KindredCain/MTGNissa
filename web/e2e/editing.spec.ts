import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
test("failed filters preserve the current results and submitted URL", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/sets");
  await expect(page.locator("tbody tr")).toHaveCount(2);
  state.fail_lists = true;
  await page
    .getByRole("textbox", { name: "按系列名称或缩写搜索" })
    .fill("failed");
  await page
    .getByRole("textbox", { name: "按系列名称或缩写搜索" })
    .press("Enter");
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(page).toHaveURL("/sets");
  await expect(page.locator("tbody tr")).toHaveCount(2);
});
test("quick drag moves one copy and held drag moves the whole source stack", async ({
  page,
}) => {
  await mock_api(page);
  for (const held of [false, true]) {
    await page.goto("/decks/1/edit");
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
    const source = page.getByRole("button", { name: "测试卡牌 1，2 张" });
    await expect(source).toBeVisible();
    const region = page.locator("section").filter({
      has: page.getByRole("heading", { name: "备牌", exact: true }),
    });
    const source_box = (await source.boundingBox())!,
      target = (await region.boundingBox())!;
    await page.mouse.move(source_box.x + 75, source_box.y + 90);
    await page.mouse.down();
    if (held) await page.waitForTimeout(400);
    await page.mouse.move(source_box.x + 81, source_box.y + 90);
    await page.mouse.move(target.x + 420, target.y + 100, { steps: 8 });
    await page.mouse.up();
    await expect(
      region.getByRole("button", { name: `测试卡牌 1，${held ? 2 : 1} 张` }),
    ).toBeVisible();
    expect(
      await page
        .getByRole("textbox", { name: "卡组代码", exact: true })
        .count(),
    ).toBe(0);
  }
});
