import { expect, test } from "@playwright/test";
import { mock_api } from "./mock-api";
import { detail, uuid } from "../src/test/fixtures";

for (const count of [0, 1, 12]) {
  test(`version scrollbars appear on interaction without arrows (${count} versions)`, async ({
    page,
  }) => {
    await mock_api(page);
    const data = detail();
    data.other_versions = Array.from({ length: count }, (_, index) => ({
      set: {
        ...data.card.set,
        code: `T${index}`,
        display_name: `测试版本 ${index + 1}`,
      },
      collector_number: String(index + 1),
      released_at: data.card.released_at,
      normal_quantity: 0,
      foil_quantity: 0,
      languages: data.languages,
    }));
    await page.route(`**/api/v1/cards/${uuid(1)}`, (route) =>
      route.fulfill({
        json: { status: 200, data, error: null, request_id: "test" },
      }),
    );
    await page.goto(`/cards/${uuid(1)}`);
    const versions = page.getByRole("region", {
      name: "其他系列版本",
      exact: true,
    });
    await expect(versions).toBeVisible();
    const thumbColor = () =>
      versions.evaluate(
        (element) =>
          getComputedStyle(element, "::-webkit-scrollbar-thumb")
            .backgroundColor,
      );
    expect(await thumbColor()).toBe("rgba(0, 0, 0, 0)");
    const geometry = await versions.evaluate((element) => ({
      overflow: getComputedStyle(element).overflowY,
      overflowing: element.scrollHeight > element.clientHeight,
      buttons: getComputedStyle(element, "::-webkit-scrollbar-button").display,
    }));
    expect(geometry.overflow).toBe("auto");
    expect(geometry.overflowing).toBe(count === 12);
    expect(geometry.buttons).toBe("none");
    await versions.hover();
    expect(await thumbColor()).not.toBe("rgba(0, 0, 0, 0)");
    if (count === 12) {
      const top = await page.locator("body").evaluate((body) => body.scrollTop);
      await page.mouse.wheel(0, 80);
      await expect
        .poll(() => versions.evaluate((element) => element.scrollTop))
        .toBeGreaterThan(0);
      expect(
        await page.locator("body").evaluate((body) => body.scrollTop),
      ).toBe(top);
    }
    await page.mouse.move(0, 0);
    expect(await thumbColor()).toBe("rgba(0, 0, 0, 0)");
    await versions.focus();
    expect(await thumbColor()).not.toBe("rgba(0, 0, 0, 0)");
  });
}

test("overlay scrollbars preserve page width and support wheel, keys and dragging", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/sets");
  await expect(page.getByRole("heading", { name: "全部卡牌" })).toBeVisible();
  await expect(page.getByRole("scrollbar")).toHaveCount(0);
  const width = await page.locator("body").evaluate((body) => body.clientWidth);
  await page.getByRole("button", { name: "英文", exact: true }).click();
  const bar = page.getByRole("scrollbar", { name: "页面纵向滚动" });
  await expect(bar).toBeVisible();
  expect(await page.locator("body").evaluate((body) => body.clientWidth)).toBe(
    width,
  );
  expect(
    await page
      .locator("body")
      .evaluate((body) => body.clientWidth === innerWidth),
  ).toBe(true);
  expect(
    await bar.evaluate((element) => getComputedStyle(element).backgroundColor),
  ).toBe("rgba(0, 0, 0, 0)");
  await page.screenshot({
    path: `test-results/${test.info().project.name}-overlay-scrollbar.png`,
  });

  await page.mouse.move(600, 500);
  await page.mouse.wheel(0, 200);
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollTop))
    .toBeGreaterThan(0);
  await bar.focus();
  await bar.press("Home");
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollTop))
    .toBe(0);
  await bar.press("PageDown");
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollTop))
    .toBeGreaterThan(100);
  await bar.press("Home");
  await expect(bar).toHaveAttribute("aria-valuenow", "0");
  const box = (await bar.boundingBox())!;
  const thumb = await bar.evaluate((element) =>
    parseFloat(getComputedStyle(element).getPropertyValue("--thumb-size")),
  );
  await page.mouse.move(box.x + box.width / 2, box.y + thumb / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.7, {
    steps: 8,
  });
  await page.mouse.up();
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollTop))
    .toBeGreaterThan(100);
  await page.goBack();
  await expect(page.getByRole("scrollbar")).toHaveCount(0);
  expect(await page.locator("body").evaluate((body) => body.clientWidth)).toBe(
    width,
  );
});

test("overlay horizontal scrolling preserves the minimum page width", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1100, height: 720 });
  await mock_api(page);
  await page.goto("/sets");
  const bar = page.getByRole("scrollbar", { name: "页面横向滚动" });
  await expect(bar).toBeVisible();
  await bar.focus();
  await bar.press("End");
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollLeft))
    .toBeGreaterThan(0);
  const box = (await bar.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(1100);
  expect(
    await page.locator("#root").evaluate((root) => root.clientWidth),
  ).toBeGreaterThanOrEqual(1280);
  await bar.press("Home");
  await expect
    .poll(() => page.locator("body").evaluate((body) => body.scrollLeft))
    .toBe(0);
});

test("modal locks hide page scrollbars and restore them after closing", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.fail_adjustment = true;
  await page.goto("/collection?merge_languages=false");
  await page.addStyleTag({ content: "#root { min-height: 2200px; }" });
  const bar = page.getByRole("scrollbar", { name: "页面纵向滚动" });
  await expect(bar).toBeVisible();
  const width = await page.locator("body").evaluate((body) => body.clientWidth);
  await page.getByRole("button", { name: "增加普通", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.locator('input[role="scrollbar"]')).toHaveCount(0);
  const top = await page.locator("body").evaluate((body) => body.scrollTop);
  await page.mouse.move(20, 400);
  await page.mouse.wheel(0, 500);
  expect(await page.locator("body").evaluate((body) => body.scrollTop)).toBe(
    top,
  );
  expect(await page.locator("body").evaluate((body) => body.clientWidth)).toBe(
    width,
  );
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(bar).toBeVisible();
  expect(await page.locator("body").evaluate((body) => body.scrollTop)).toBe(
    top,
  );
});
