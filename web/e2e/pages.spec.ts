import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { uuid } from "../src/test/fixtures";
test("series navigation, local filters and desktop card columns", async ({
  page,
}, info) => {
  const state = await mock_api(page);
  await page.goto("/sets");
  await expect(page.getByRole("heading", { name: "全部卡牌" })).toBeVisible();
  const unavailable = page.getByRole("button", { name: "日文", exact: true });
  await expect(unavailable).toBeDisabled();
  await unavailable.click({ force: true });
  await expect(page).toHaveURL(/\/sets$/);
  await page.getByRole("button", { name: "英文", exact: true }).click();
  await expect(page).toHaveURL(/language=en/);
  await expect(page.locator("article").first()).toBeVisible();
  if (info.project.name.startsWith("desktop")) {
    const top = await page
      .locator("article")
      .evaluateAll((items) =>
        items.slice(0, 6).map((item) => item.getBoundingClientRect().top),
      );
    const columns = info.project.name === "desktop-1440" ? 5 : 4;
    expect(top.slice(0, columns).every((value) => value === top[0])).toBe(true);
    expect(top[columns]).toBeGreaterThan(top[0]);
  }
  const requests = state.cards_requests;
  await page.getByRole("button", { name: "法术力", exact: true }).click();
  await page.getByRole("checkbox", { name: "1", exact: true }).click();
  await page.getByRole("heading", { name: "全部卡牌" }).click();
  await expect(page).toHaveURL(/mana_value=1/);
  expect(state.cards_requests).toBe(requests);
  await page.screenshot({
    path: `test-results/${info.project.name}-cards.png`,
  });
});
test("card details flip face without refetching and preserve tag queries", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto(`/cards/${uuid(1)}`);
  await expect(page.getByRole("heading", { name: "卡牌正面" })).toBeVisible();
  await page.getByRole("button", { name: "查看反面", exact: true }).click();
  await expect(page.getByRole("heading", { name: "卡牌背面" })).toBeVisible();
  await page.getByRole("button", { name: "功能标签 1" }).click();
  await page.getByRole("link", { name: 'otag:"mana rock"' }).click();
  await expect(page).toHaveURL(/\/search\?q=/);
  expect(new URL(page.url()).searchParams.get("q")).toBe('otag:"mana rock"');
  expect(state.cards_requests).toBe(0);
});
test("collection locks actions until confirmed and keeps failed quantities", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.delay_adjustment = 500;
  await page.goto("/collection?merge_languages=false");
  const increase = page.getByRole("button", { name: "增加普通卡" });
  await increase.click();
  await expect(increase).toBeDisabled();
  await expect(page.getByRole("button", { name: "增加闪卡" })).toBeDisabled();
  await expect(page.locator("output").first()).toHaveText("3");
  expect(state.adjustments).toBe(1);
  state.fail_adjustment = true;
  await increase.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog")).toContainText("没有足够的卡牌");
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(page.locator("output").first()).toHaveText("3");
  await page.goto("/collection");
  await expect(page.getByRole("button", { name: "增加普通卡" })).toHaveCount(0);
});
test("wishlist cancellation retains its card and can be restored", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/wishlist");
  await page.getByRole("button", { name: "★ 移出愿望单" }).click();
  await expect(
    page.getByRole("button", { name: "☆ 恢复愿望单" }),
  ).toBeVisible();
  expect(state.included).toBe(false);
  await expect(page.locator("article")).toHaveCount(1);
  await page.getByRole("button", { name: "☆ 恢复愿望单" }).click();
  await expect(
    page.getByRole("button", { name: "★ 移出愿望单" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "★ 移出愿望单" }).click();
  await expect(
    page.getByRole("button", { name: "☆ 恢复愿望单" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "全部卡牌", exact: true }).click();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await page.getByRole("link", { name: "愿望单", exact: true }).click();
  await expect(page.locator("article")).toHaveCount(0);
});
test("deck editor imports atomically, preserves names and warns before leaving", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/decks/1/edit");
  await page
    .getByRole("textbox", { name: "卡组名称", exact: true })
    .fill("未保存名称");
  await page
    .getByRole("textbox", { name: "卡组代码", exact: true })
    .fill("Deck\n3 Test Card 2");
  state.fail_context = true;
  await page.getByRole("button", { name: "读取卡组配置" }).click();
  await expect(page.getByRole("dialog")).toContainText("发生意外错误");
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "测试卡牌 1，2 张" }),
  ).toBeVisible();
  state.fail_context = false;
  await page.getByRole("button", { name: "读取卡组配置" }).click();
  await expect(
    page.getByRole("button", { name: "测试卡牌 2，3 张" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "卡组名称", exact: true }),
  ).toHaveValue("未保存名称");
  await page.getByRole("link", { name: "操作日志", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("放弃尚未保存");
  await page.getByRole("button", { name: "继续编辑" }).click();
  await page.getByRole("button", { name: "保存卡组", exact: true }).click();
  await expect(page).toHaveURL("/decks/1");
  expect(state.save_body?.name).toBe("未保存名称");
  expect(JSON.stringify(state.save_body)).not.toContain("source_text");
});
test("add-card overlay and statistics keep background locked", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  const modal = page.getByRole("dialog");
  await expect(modal).toBeVisible();
  await expect(page.locator("body")).toHaveAttribute("data-scroll-locked", "1");
  await modal.getByRole("button", { name: "搜索", exact: true }).click();
  await expect(
    modal.getByRole("button", { name: "加入", exact: true }),
  ).toBeVisible();
  await modal.getByRole("button", { name: "加入", exact: true }).click();
  await modal.getByRole("button", { name: "×", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "测试卡牌 2，1 张" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "卡组统计", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "法术力曲线", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "卡组统计", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
test("logs use read-only snapshots and retain summary when closing details", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/operation_logs");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("历史卡牌名称");
  await expect(page.getByRole("dialog")).toContainText("12:00:00");
  await expect(page.getByRole("dialog").getByRole("link")).toHaveCount(0);
  await page.getByRole("button", { name: "×", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "增加卡牌", exact: true }),
  ).toBeVisible();
});
test("all page deep links render without script crashes", async ({ page }) => {
  await mock_api(page);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  for (const route of [
    "/sets",
    "/sets/KLD/cards?language=en",
    "/sets/TKLD/tokens",
    "/search?q=test",
    `/cards/${uuid(1)}`,
    "/collection",
    "/wishlist",
    "/decks",
    "/decks/1",
    "/decks/1/edit",
    "/operation_logs",
  ]) {
    await page.goto(route);
    await expect(page.locator("main")).toBeVisible();
    await expect(page.getByText("页面暂时无法显示")).toHaveCount(0);
    await expect(
      page
        .locator('article, tbody tr, input[aria-label="卡组名称"], h1')
        .first(),
    ).toBeVisible();
  }
  expect(errors).toEqual([]);
});
