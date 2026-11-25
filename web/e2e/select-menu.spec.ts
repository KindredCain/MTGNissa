import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";

test("sort menu uses the shared styling and supports keyboard selection and dismissal", async ({
  page,
}, testInfo) => {
  await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  const select = page.getByRole("combobox", { name: "排序", exact: true });
  const menu = page.getByRole("listbox");
  await select.focus();
  await select.press("Enter");
  await expect(menu).toBeVisible();
  await expect(menu).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await expect(menu).toHaveCSS("border-radius", "8px");
  await expect(menu).toHaveCSS("border-color", "rgb(214, 219, 214)");
  const selected = page.getByRole("option", {
    name: "收藏编号 ↑",
    exact: true,
  });
  await expect(selected).toHaveAttribute("aria-selected", "true");
  await expect(selected.locator("svg")).toBeVisible();
  await page.keyboard.press("End");
  await expect(
    page.getByRole("option", { name: "名称 ↑", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(select).toBeFocused();
  await expect(select).toContainText("收藏编号 ↑");
  await expect(page).toHaveURL("/sets/KLD/cards?language=en");
  await select.click();
  await page.mouse.click(10, 10);
  await expect(menu).toHaveCount(0);
  await expect(select).toContainText("收藏编号 ↑");
  await select.click();
  await page.screenshot({ path: testInfo.outputPath("sort-menu.png") });
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await expect(menu).toHaveCount(0);
  await expect(select).toContainText("名称 ↑");
  await expect(page).toHaveURL(/sort=name_asc/);
});

test("a failed remote sort retains the selected value and submitted URL", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/decks?q=test");
  const select = page.getByRole("combobox", { name: "卡组排序", exact: true });
  await expect(select).toContainText("更新时间 ↓");
  state.fail_lists = true;
  await select.click();
  await page.getByRole("option", { name: "名称 ↑", exact: true }).click();
  const error = page.getByRole("dialog", { name: "操作失败", exact: true });
  await expect(error).toBeVisible();
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await error.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(select).toContainText("更新时间 ↓");
  await expect(page).toHaveURL("/decks?q=test");
  state.fail_lists = false;
  await select.click();
  await page.getByRole("option", { name: "名称 ↑", exact: true }).click();
  await expect(select).toContainText("名称 ↑");
  await expect(page).toHaveURL(/sort=name_asc/);
});

test("a short desktop viewport keeps the sort menu inside the viewport and scrolls its options", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 420 });
  await mock_api(page);
  await page.goto("/sets");
  const select = page.getByRole("combobox", { name: "系列排序", exact: true });
  await select.click();
  const menu = page.getByRole("listbox");
  await expect(menu).toBeVisible();
  const bounds = (await menu.boundingBox())!;
  expect(bounds.x).toBeGreaterThanOrEqual(8);
  expect(bounds.y).toBeGreaterThanOrEqual(8);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(1272);
  expect(bounds.y + bounds.height).toBeLessThanOrEqual(412);
  await expect
    .poll(() =>
      menu
        .locator("[data-radix-select-viewport]")
        .evaluate((element) => element.scrollHeight > element.clientHeight),
    )
    .toBe(true);
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/sort=card_count_asc/);
});

for (const route of ["/decks/1", "/decks/1/edit"]) {
  test(`statistics menus keep the drawer open while changing groups at ${route}`, async ({
    page,
  }, testInfo) => {
    await mock_api(page);
    await page.goto(route);
    await page.getByRole("button", { name: "卡组统计", exact: true }).click();
    const drawer = page.getByRole("dialog", { name: "卡组统计", exact: true });
    for (const [label, option] of [
      ["法术力曲线分组", "颜色"],
      ["概率分组", "产费颜色"],
      ["数量分布分组", "功能标签"],
    ]) {
      const select = drawer.getByRole("combobox", { name: label, exact: true });
      await select.scrollIntoViewIfNeeded();
      const heading = (await select
        .locator("..")
        .getByRole("heading")
        .boundingBox())!;
      await select.click();
      const menu = page.getByRole("listbox");
      await expect(menu).toBeVisible();
      await expect(drawer).toHaveAttribute("data-motion", "open");
      await page.mouse.click(heading.x + 10, heading.y + heading.height / 2);
      await expect(menu).toHaveCount(0);
      await expect(drawer).toHaveAttribute("data-motion", "open");
      await select.click();
      await expect(menu).toHaveCSS("background-color", "rgb(32, 59, 43)");
      await expect(menu).toHaveCSS("border-color", "rgb(59, 88, 70)");
      await expect(menu).toHaveCSS("color", "rgb(198, 214, 203)");
      const selected = menu.locator('[data-state="checked"]');
      await expect(selected).toHaveCSS("color", "rgb(168, 213, 184)");
      await expect(selected.locator("svg")).toHaveCSS(
        "color",
        "rgb(168, 213, 184)",
      );
      await menu.getByRole("option", { name: option, exact: true }).click();
      await expect(select).toContainText(option);
      await expect(drawer).toHaveAttribute("data-motion", "open");
      await expect(menu).toHaveCount(0);
      await select.click();
      if (label === "概率分组")
        await page.screenshot({
          path: testInfo.outputPath("statistics-menu.png"),
        });
      await page.keyboard.press("Escape");
      await expect(menu).toHaveCount(0);
      await expect(drawer).toHaveAttribute("data-motion", "open");
      await expect(select).toBeFocused();
    }
    await drawer.getByRole("combobox").first().click();
    await expect(page.getByRole("listbox")).toBeVisible();
    await expect(drawer).toHaveCSS("pointer-events", "none");
    await page.mouse.click(8, 100);
    await expect(page.getByRole("listbox")).toHaveCount(0);
    await expect(drawer).toHaveAttribute("data-motion", "open");
    await expect(drawer).toHaveCSS("pointer-events", "auto");
    await page.mouse.click(8, 100);
    await expect(drawer).toHaveCount(0);
  });
}
