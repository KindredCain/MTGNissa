import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
test("error modal stays inside the active viewport", async ({ page }) => {
  const state = await mock_api(page);
  state.fail_adjustment = true;
  await page.goto("/collection?merge_languages=false");
  await page.getByRole("button", { name: "增加普通卡" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  const geometry = await page.evaluate(() => {
    const modal = document.querySelector("[role=dialog]")!,
      rect = modal.getBoundingClientRect(),
      close = modal.querySelector("button")!.getBoundingClientRect();
    return {
      width: innerWidth,
      height: innerHeight,
      x: scrollX,
      y: scrollY,
      visual: {
        width: visualViewport?.width,
        height: visualViewport?.height,
        x: visualViewport?.offsetLeft,
        y: visualViewport?.offsetTop,
        scale: visualViewport?.scale,
      },
      modal: {
        left: rect.left,
        top: rect.top,
        width: rect.width,
        height: rect.height,
      },
      close: { left: close.left, top: close.top },
    };
  });
  expect(geometry.modal.top).toBeGreaterThanOrEqual(0);
  expect(geometry.modal.left).toBeGreaterThanOrEqual(geometry.visual.x ?? 0);
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
