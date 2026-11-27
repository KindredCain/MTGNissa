import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";

test("probability stepper fields align inside one dark control", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/decks/1");
  await page.getByRole("button", { name: "卡组统计", exact: true }).click();
  const drawer = page.getByRole("dialog", { name: "卡组统计", exact: true });
  await expect(drawer).toHaveAttribute("data-motion", "open");
  for (const label of ["至少张数", "抽牌张数"]) {
    const field = drawer.getByRole("spinbutton", { name: label, exact: true });
    await field.scrollIntoViewIfNeeded();
    const group = field.locator("..");
    const boxes = await group.evaluate((element) => {
      const children = Array.from(element.children, (child) =>
        child.getBoundingClientRect(),
      );
      return children.map((box) => ({
        top: box.top,
        height: box.height,
        left: box.left,
        right: box.right,
      }));
    });
    expect(boxes).toHaveLength(3);
    for (const box of boxes) {
      expect(box.top).toBeCloseTo(boxes[0].top, 0);
      expect(box.height).toBeCloseTo(boxes[0].height, 0);
    }
    expect(boxes[0].right).toBeCloseTo(boxes[1].left, 0);
    expect(boxes[1].right).toBeCloseTo(boxes[2].left, 0);
    await expect(field).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  }
});

for (const route of ["/decks/1", "/decks/1/edit"]) {
  for (const closing of ["bookmark", "backdrop"] as const) {
    test(`statistics slides with its bookmark and restores the trigger after ${closing} closes ${route}`, async ({
      page,
    }) => {
      await mock_api(page);
      await page.goto(route);
      // Control the transition clock so its middle and endpoint can be inspected.
      await page.addStyleTag({
        content: "[data-motion] { transition-duration: 10s !important; }",
      });
      const trigger = page.locator('button[class*="drawerButton"]');
      await expect(trigger).toBeVisible();
      const origin = (await trigger.boundingBox())!;
      const scroll_top = await page.locator("body").evaluate((element) => {
        element.scrollTop = 120;
        return element.scrollTop;
      });
      await trigger.click();
      const drawer = page.getByRole("dialog", {
        name: "卡组统计",
        exact: true,
      });
      await expect(drawer).toHaveAttribute("data-motion", "open");
      await expect
        .poll(() =>
          drawer.evaluate((element) => element.getAnimations().length),
        )
        .toBe(1);
      await expect(trigger).toBeHidden();
      const entrance = await drawer.evaluate((element) => {
        const animation = element.getAnimations()[0];
        animation.pause();
        animation.currentTime = 0;
        const start = element
          .querySelector("button")!
          .getBoundingClientRect()
          .toJSON();
        animation.currentTime = 5000;
        const middle = element
          .querySelector("button")!
          .getBoundingClientRect()
          .toJSON();
        animation.finish();
        return { start, middle };
      });
      expect(entrance.middle.x).toBeLessThan(entrance.start.x);
      expect(entrance.start.y).toBeCloseTo(origin.y, 1);
      expect(entrance.middle.y).toBeCloseTo(origin.y, 1);
      await expect
        .poll(() =>
          drawer.evaluate((element) => element.getAnimations().length),
        )
        .toBe(0);
      const bookmark = drawer.getByRole("button", {
        name: "卡组统计",
        exact: true,
      });
      expect((await bookmark.boundingBox())!.y).toBeCloseTo(origin.y, 1);
      const overlay = drawer.locator("..");
      if (closing === "backdrop") {
        await overlay.evaluate((element) => {
          element.scrollTop = element.scrollHeight;
        });
        await expect
          .poll(() => overlay.evaluate((element) => element.scrollTop))
          .toBeGreaterThan(0);
        expect((await bookmark.boundingBox())!.y).toBeLessThan(0);
        await page.mouse.click(8, 100);
      } else await bookmark.click();
      await expect(drawer).toHaveAttribute("data-motion", "closing");
      await expect
        .poll(() =>
          drawer.evaluate((element) => element.getAnimations().length),
        )
        .toBe(1);
      const middle = await drawer.evaluate((element) => {
        const animation = element.getAnimations()[0];
        animation.pause();
        animation.currentTime = 0;
        const start = element
          .querySelector("button")!
          .getBoundingClientRect()
          .toJSON();
        animation.currentTime = 5000;
        return {
          start,
          middle: element
            .querySelector("button")!
            .getBoundingClientRect()
            .toJSON(),
        };
      });
      expect(middle.middle.x).toBeGreaterThan(middle.start.x);
      expect(middle.middle.y).toBeCloseTo(middle.start.y, 1);
      await expect(trigger).toBeHidden();
      await expect(page.locator("body")).toHaveAttribute(
        "data-scroll-locked",
        "1",
      );
      const exit = await drawer.evaluate((element) => {
        const animation = element.getAnimations()[0];
        animation.currentTime = 10000;
        const bookmark = element
          .querySelector("button")!
          .getBoundingClientRect();
        const left = element.getBoundingClientRect().left;
        const overlay = element.parentElement!;
        const right =
          overlay.getBoundingClientRect().left + overlay.clientWidth;
        animation.finish();
        return {
          left,
          right,
          bookmark_left: bookmark.left,
          bookmark_right: bookmark.right,
        };
      });
      expect(exit.left).toBeCloseTo(exit.right, 1);
      expect(exit.bookmark_left).toBeCloseTo(exit.right - origin.width, 1);
      expect(exit.bookmark_right).toBeCloseTo(exit.right, 1);
      await expect(drawer).toHaveCount(0);
      await expect(trigger).toBeVisible();
      await expect(trigger).toBeFocused();
      await expect(page.locator("body")).not.toHaveAttribute(
        "data-scroll-locked",
        "1",
      );
      expect((await trigger.boundingBox())!.y).toBeCloseTo(origin.y, 1);
      await expect
        .poll(() =>
          page.locator("body").evaluate((element) => element.scrollTop),
        )
        .toBe(scroll_top);
    });
  }
}

test("closing during entry reverses from the current position", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/decks/1/edit");
  await page.addStyleTag({
    content: "[data-motion] { transition-duration: 10s !important; }",
  });
  const trigger = page.locator('button[class*="drawerButton"]');
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "卡组统计", exact: true });
  await expect(drawer).toHaveAttribute("data-motion", "open");
  await expect
    .poll(() => drawer.evaluate((element) => element.getAnimations().length))
    .toBe(1);
  const left = await drawer.evaluate((element) => {
    const animation = element.getAnimations()[0];
    animation.pause();
    animation.currentTime = 3000;
    return element.getBoundingClientRect().left;
  });
  await page.mouse.click(8, 100);
  await expect(drawer).toHaveAttribute("data-motion", "closing");
  const closing_left = await drawer.evaluate((element) => {
    const animation = element.getAnimations()[0];
    animation.pause();
    animation.currentTime = 0;
    const left = element.getBoundingClientRect().left;
    animation.finish();
    return left;
  });
  expect(closing_left).toBeCloseTo(left, 0);
  await expect(drawer).toHaveCount(0);
  await expect(trigger).toBeVisible();
});

test("reduced motion still completes closing and restores the bookmark", async ({
  page,
}) => {
  await mock_api(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/decks/1");
  const trigger = page.locator('button[class*="drawerButton"]');
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "卡组统计", exact: true });
  await expect(drawer).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(trigger).toBeVisible();
});
