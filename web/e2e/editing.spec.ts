import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { context, uuid } from "../src/test/fixtures";
import { context_card, with_summary } from "../src/decks/model";

for (const operation of ["quantity", "add", "move"] as const) {
  test(`store validation feedback is presented once for ${operation}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    if (operation === "add")
      state.deck.sections[1]!.cards = [context_card(context(2), 4294967295)];
    else if (operation === "quantity")
      state.deck.sections[1]!.cards[0]!.quantity = 4294967295;
    else state.deck.sections[2]!.cards = [context_card(context(), 4294967295)];
    state.deck = with_summary(state.deck);
    await page.goto("/decks/1/edit");
    const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
    if (operation === "add") {
      await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
      await modal.getByRole("button", { name: "搜索", exact: true }).click();
      await modal.getByRole("button", { name: "加入", exact: true }).click();
    } else {
      const source = page.getByRole("button", {
        name: `测试卡牌 1，${operation === "quantity" ? 4294967295 : 2} 张`,
        exact: true,
      });
      await source.hover({ position: { x: 70, y: 12 } });
      if (operation === "quantity")
        await page
          .getByRole("button", { name: "增加测试卡牌 1", exact: true })
          .click();
      else {
        const region = page.locator("section").filter({
          has: page.getByRole("heading", { name: "备牌", exact: true }),
        });
        const source_box = (await source.boundingBox())!,
          target = (await region.boundingBox())!;
        await page.mouse.move(source_box.x + 75, source_box.y + 90);
        await page.mouse.down();
        await page.mouse.move(source_box.x + 81, source_box.y + 90);
        await page.mouse.move(target.x + 420, target.y + 24, { steps: 8 });
        await page.mouse.up();
      }
    }
    const error = page.getByRole("dialog", { name: "操作失败", exact: true });
    await expect(error).toBeVisible();
    await expect(error).toContainText("单张卡牌数量不能超过 4,294,967,295。");
    await error.getByRole("button", { name: "关闭", exact: true }).click();
    await expect(error).toHaveCount(0);
    if (operation === "add")
      await modal.getByRole("button", { name: "×", exact: true }).click();
    await expect(
      page.getByRole("button", {
        name: `测试卡牌 ${operation === "add" ? 2 : 1}，4294967295 张`,
        exact: true,
      }),
    ).toBeVisible();
    if (operation === "move")
      await expect(
        page.getByRole("button", { name: "测试卡牌 1，2 张", exact: true }),
      ).toBeVisible();
  });
}

for (const kind of ["add", "error", "leave"] as const) {
  test(`modal ${kind} blocks background cover Escape`, async ({ page }) => {
    await mock_api(page);
    await page.goto("/decks/1/edit");
    const cover = page.getByRole("button", {
      name: "选择封面",
      exact: true,
      includeHidden: true,
    });
    await cover.click();
    await expect(cover).toHaveAttribute("aria-pressed", "true");

    if (kind === "add") {
      await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
      const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
      await expect(modal).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(modal).toBeVisible();
      await expect(cover).toHaveAttribute("aria-pressed", "true");

      await modal.getByRole("button", { name: "搜索", exact: true }).click();
      await modal.getByRole("spinbutton").fill("0");
      await modal.getByRole("button", { name: "加入", exact: true }).click();
      const error = page.getByRole("dialog", { name: "操作失败", exact: true });
      await expect(error).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(error).toBeVisible();
      await expect(cover).toHaveAttribute("aria-pressed", "true");
      await error.getByRole("button", { name: "关闭", exact: true }).click();
      await modal.getByRole("button", { name: "×", exact: true }).click();
    } else {
      await page
        .getByRole("textbox", { name: "卡组名称", exact: true })
        .fill(kind === "error" ? "" : "未保存名称");
      if (kind === "error")
        await page
          .getByRole("button", { name: "保存卡组", exact: true })
          .click();
      else await page.getByRole("link", { name: "卡组", exact: true }).click();
      const modal = page.getByRole("dialog", {
        name: kind === "error" ? "操作失败" : "离开卡组编辑？",
        exact: true,
      });
      await expect(modal).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(modal).toBeVisible();
      await expect(cover).toHaveAttribute("aria-pressed", "true");
      await modal
        .getByRole("button", {
          name: kind === "error" ? "关闭" : "继续编辑",
          exact: true,
        })
        .click();
    }

    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page).toHaveURL("/decks/1/edit");
    await cover.evaluate((button) => {
      button.addEventListener("keydown", (event) => event.preventDefault(), {
        once: true,
      });
      button.dispatchEvent(
        new KeyboardEvent("keydown", {
          key: "Escape",
          bubbles: true,
          cancelable: true,
        }),
      );
    });
    await expect(cover).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("Escape");
    await expect(cover).toHaveAttribute("aria-pressed", "false");
  });
}

for (const confirmation of ["Enter", "Space", "pointer"] as const) {
  test(`add-card suggestions retain internal focus and confirm with ${confirmation}`, async ({
    page,
  }) => {
    await mock_api(page);
    const searches: string[] = [];
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.pathname === "/api/v1/decks/1/cards/search")
        searches.push(url.searchParams.get("q") ?? "");
    });
    await page.route("**/api/v1/decks/1/cards/name_suggestions?**", (route) =>
      route.fulfill({
        json: {
          status: 200,
          data: {
            items: [
              { printed_name: "Alpha First" },
              { printed_name: "Alpha Second" },
            ],
          },
          error: null,
          request_id: "test",
        },
      }),
    );
    await page.goto("/decks/1/edit");
    await page.clock.install();
    await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
    const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
    const input = modal.getByRole("textbox", { name: "搜索加入卡组的卡牌" });
    const first = modal.getByRole("button", {
      name: "Alpha First",
      exact: true,
    });
    const second = modal.getByRole("button", {
      name: "Alpha Second",
      exact: true,
    });
    await input.fill("Alpha");
    await expect(first).toBeVisible();
    await input.press("Tab");
    await expect(first).toBeFocused();
    await page.clock.runFor(300);
    await expect(first).toBeFocused();
    await first.press("Tab");
    await expect(second).toBeFocused();
    await second.press("Shift+Tab");
    await expect(first).toBeFocused();
    await first.press("Shift+Tab");
    await expect(input).toBeFocused();
    await page.clock.runFor(300);
    await expect(first).toBeVisible();
    expect(searches).toEqual([]);
    await input.press("Tab");
    await first.press("Tab");
    await second.press("Tab");
    await expect(
      modal.getByRole("button", { name: "搜索", exact: true }),
    ).toBeFocused();
    await input.focus();
    await page.clock.runFor(300);
    await expect(first).toBeVisible();
    await input.press("Tab");
    await first.press("Tab");
    await second.press("Tab");
    await page.clock.runFor(300);
    await expect(
      modal.getByRole("button", { name: "搜索", exact: true }),
    ).toBeFocused();
    await expect(first).toHaveCount(0);
    await input.focus();
    await expect(second).toBeVisible();
    if (confirmation === "pointer") await second.click();
    else {
      await input.press("Tab");
      await first.press("Tab");
      await second.press(confirmation === "Space" ? "Space" : "Enter");
    }
    await expect(input).toHaveValue("Alpha Second");
    await expect.poll(() => searches).toEqual(["Alpha Second"]);
    await expect(first).toHaveCount(0);
    await expect(second).toHaveCount(0);
    await expect(
      modal.getByRole("button", { name: "测试卡牌 2", exact: true }),
    ).toBeVisible();
  });
}

test("invalid add quantities use the shared error dialog without adding cards", async ({
  page,
}) => {
  await mock_api(page);
  let context_requests = 0;
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.endsWith("/card_contexts"))
      context_requests++;
  });
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
  await modal.getByRole("button", { name: "搜索", exact: true }).click();
  const quantity = modal.getByRole("spinbutton");
  const error = page.getByRole("dialog", { name: "操作失败", exact: true });
  for (const value of ["0", "-1", "1.5", "4294967296", ""]) {
    await quantity.fill(value);
    await modal.getByRole("button", { name: "加入", exact: true }).click();
    await expect(error).toBeVisible();
    await expect(error).toContainText(
      "加入数量必须为 1 至 4,294,967,295 的整数。",
    );
    expect(context_requests).toBe(0);
    await error.getByRole("button", { name: "关闭", exact: true }).click();
  }
  await quantity.fill("1");
  await quantity.press("Enter");
  await expect.poll(() => context_requests).toBe(1);
  await expect(
    modal.getByRole("button", { name: "加入", exact: true }),
  ).toBeEnabled();
  expect(context_requests).toBe(1);
  await modal.getByRole("button", { name: "×", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "测试卡牌 2，1 张", exact: true }),
  ).toBeVisible();
});

for (const route of ["/decks/1", "/decks/1/edit"]) {
  for (const status of ["shortage", "deck_usage"] as const) {
    test(`${status} hints do not change card expansion or fixation at ${route}`, async ({
      page,
    }) => {
      const state = await mock_api(page);
      state.deck.sections[1]!.cards = [
        context_card(context(1), 2),
        context_card(
          context(2, {
            type_group: "artifact",
            owned_quantity: status === "shortage" ? 0 : 2,
            other_decks_quantity: status === "shortage" ? 0 : 2,
            other_decks_count: status === "shortage" ? 0 : 1,
          }),
          1,
        ),
        context_card(context(3), 1),
        context_card(context(4, { type_group: "artifact" }), 1),
      ];
      state.deck = with_summary(state.deck);
      await page.goto(route);
      const first = page.getByRole("button", {
          name: "测试卡牌 1，2 张",
          exact: true,
        }),
        badge = page.getByRole("button", {
          name: status === "shortage" ? "数量不足" : "卡组占用",
          exact: true,
        }),
        hint = page
          .locator("[data-deck-preview]")
          .filter({ hasText: "当前卡组" }),
        expanded = page.locator('[data-deck-card][class*="expanded"]');
      await badge.hover();
      await expect(hint).toBeVisible();
      await expect(hint).toContainText(
        `${status === "shortage" ? "数量不足" : "卡组占用"} 1`,
      );
      await expect(hint).toHaveCSS(
        "border-top-color",
        status === "shortage" ? "rgb(198, 107, 98)" : "rgb(183, 149, 81)",
      );
      const anchor = (await badge.boundingBox())!,
        panel = (await hint.boundingBox())!,
        arrow = (await hint.locator("svg").first().boundingBox())!;
      expect(
        Math.abs(arrow.y + arrow.height / 2 - anchor.y - anchor.height / 2),
      ).toBeLessThanOrEqual(1);
      expect(
        Math.abs(panel.x - anchor.x - anchor.width - 4),
      ).toBeLessThanOrEqual(1);
      await expect(expanded).toHaveCount(0);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await badge.focus();
      await badge.click();
      await expect(page).toHaveURL(route);
      await expect(expanded).toHaveCount(0);
      await hint.hover();
      await page.waitForTimeout(200);
      await expect(hint).toBeVisible();
      await expect(expanded).toHaveCount(0);
      await page.mouse.move(1, 1);
      expect(await hint.count()).toBe(0);

      await first.hover({ position: { x: 70, y: 12 } });
      await expect(expanded).toHaveCount(1);
      await badge.hover();
      await expect(hint).toBeVisible();
      await hint.hover();
      await page.waitForTimeout(200);
      await expect(expanded).toHaveCount(0);
      await expect(hint).toBeVisible();
      await page.mouse.move(1, 1);

      if (route.endsWith("/edit")) {
        await first.click({ position: { x: 70, y: 12 } });
        await page.mouse.move(1, 1);
        await expect(expanded).toHaveCount(1);
        await badge.hover();
        await badge.focus();
        await badge.click();
        await hint.hover();
        await page.waitForTimeout(200);
        await expect(expanded).toHaveCount(1);
        await expect(
          expanded.getByRole("button", {
            name: "测试卡牌 1，2 张",
            exact: true,
          }),
        ).toBeVisible();
        await page.mouse.move(1, 1);
        expect(await hint.count()).toBe(0);
        await expect(expanded).toHaveCount(1);
      }
    });
  }
}

test("exception hints opening left remain connected to the status tab", async ({
  page,
}, testInfo) => {
  const state = await mock_api(page);
  state.deck.sections[1]!.cards = [
    context_card(context(2, { type_group: "land", owned_quantity: 0 }), 1),
  ];
  state.deck = with_summary(state.deck);
  await page.goto("/decks/1");
  const badge = page.getByRole("button", { name: "数量不足", exact: true });
  await badge.hover();
  const hint = page.locator("[data-exception-side]");
  await expect(hint).toHaveAttribute("data-exception-side", "left");
  const anchor = (await badge.boundingBox())!,
    panel = (await hint.boundingBox())!,
    arrow = (await hint.locator("svg").first().boundingBox())!;
  expect(Math.abs(anchor.x - panel.x - panel.width - 4)).toBeLessThanOrEqual(1);
  expect(
    Math.abs(arrow.y + arrow.height / 2 - anchor.y - anchor.height / 2),
  ).toBeLessThanOrEqual(1);
  for (const cell of await hint
    .locator('[class*="exceptionMetrics"] > div')
    .all())
    expect(
      await cell.evaluate(
        (element) => element.scrollWidth <= element.clientWidth,
      ),
    ).toBe(true);
  await hint.screenshot({ path: testInfo.outputPath("exception-left.png") });
  await page.screenshot({
    path: testInfo.outputPath("exception-connection.png"),
    clip: {
      x: panel.x - 8,
      y: panel.y - 8,
      width: panel.width + 36,
      height: panel.height + 16,
    },
  });
});

test("editor pencil follows the name without overflowing long names", async ({
  page,
}, testInfo) => {
  await mock_api(page);
  await page.goto("/decks/1/edit");
  const name = page.getByRole("textbox", { name: "卡组名称", exact: true });
  const field = name.locator("../..");
  const pencil = field.locator("svg");
  await name.fill("短名");
  const input_box = (await name.boundingBox())!,
    icon_box = (await pencil.boundingBox())!,
    field_box = (await field.boundingBox())!;
  expect(
    Math.abs(icon_box.x - input_box.x - input_box.width - 8),
  ).toBeLessThanOrEqual(1);
  expect(icon_box.width).toBe(15);
  for (const position of [
    { x: field_box.width - 5, y: field_box.height / 2 },
    {
      x: icon_box.x - field_box.x + icon_box.width / 2,
      y: field_box.height / 2,
    },
    { x: 5, y: field_box.height / 2 },
  ]) {
    await name.blur();
    await field.click({ position });
    await expect(name).toBeFocused();
  }
  await field.screenshot({ path: testInfo.outputPath("name-pencil.png") });
  await name.fill("长名称".repeat(85));
  const long_icon = (await pencil.boundingBox())!,
    long_field = (await field.boundingBox())!;
  expect(long_field.width).toBe(field_box.width);
  expect(long_icon.x + long_icon.width).toBeLessThanOrEqual(
    long_field.x + long_field.width - 10,
  );
  await expect(name).toHaveValue("长名称".repeat(85));
});

test("editor keeps one expanded card and clears the previous fixed selection", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.deck.sections[1]!.cards.push(
    context_card(context(2, { type_group: "artifact" }), 1),
    context_card(context(3), 1),
  );
  await page.goto("/decks/1/edit");
  const first = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    }),
    second = page.getByRole("button", {
      name: "测试卡牌 2，1 张",
      exact: true,
    }),
    expanded = page.locator('[data-deck-card][class*="expanded"]'),
    preview = page.getByRole("tooltip");
  await first.click({ position: { x: 70, y: 12 } });
  await first.click({ position: { x: 70, y: 12 } });
  await expect(expanded).toHaveCount(1);
  await expect(
    expanded.getByRole("button", { name: "测试卡牌 1，2 张", exact: true }),
  ).toBeVisible();
  await expect(preview).toContainText("测试卡牌 1");
  await page.mouse.move(0, 0);
  await expect(expanded).toHaveCount(0);
  await expect(preview).toHaveCount(0);
  await first.click({ position: { x: 70, y: 12 } });
  await page.mouse.move(0, 0);
  await expect(expanded).toHaveCount(1);
  await expect(preview).toHaveCount(0);
  await first.hover({ position: { x: 70, y: 12 } });
  await expect(preview).toContainText("测试卡牌 1");
  await second.hover();
  await expect(expanded).toHaveCount(1);
  await expect(
    expanded.getByRole("button", { name: "测试卡牌 2，1 张", exact: true }),
  ).toBeVisible();
  await expect(preview).toContainText("测试卡牌 2");
  await preview.getByLabel("卡牌说明").hover();
  await page.waitForTimeout(200);
  await expect(expanded).toHaveCount(1);
  await expect(preview).toContainText("测试卡牌 2");
  await page.mouse.move(0, 0);
  expect(await preview.count()).toBe(0);
  await expect(expanded).toHaveCount(0);
  await first.click({ position: { x: 70, y: 12 } });
  await page.mouse.click(1, 1);
  await expect(expanded).toHaveCount(0);
  await expect(preview).toHaveCount(0);
});

for (const path of ["/decks/1", "/decks/1/edit"]) {
  test(`draw probability excludes commanders at ${path}`, async ({ page }) => {
    const state = await mock_api(page);
    state.deck.sections[0]!.cards = [
      context_card(
        context(2, {
          statistics: { ...context(2).statistics, type_groups: ["creature"] },
        }),
        3,
      ),
    ];
    state.deck.sections[1]!.cards = [
      context_card(
        context(1, {
          statistics: { ...context(1).statistics, type_groups: ["artifact"] },
        }),
        7,
      ),
    ];
    state.deck = with_summary(state.deck);
    await page.goto(path);
    await page.getByRole("button", { name: "卡组统计", exact: true }).click();
    const probability = page.locator("section").filter({
      has: page.getByRole("heading", { name: "抽到概率", exact: true }),
    });
    await expect(probability.getByRole("row")).toHaveCount(2);
    await expect(probability.getByRole("row").last()).toHaveText(
      "神器7100.00%",
    );
    await expect(
      probability.getByRole("spinbutton", { name: "抽牌张数", exact: true }),
    ).toHaveAttribute("max", "7");
    await expect(
      probability.getByRole("spinbutton", { name: "至少张数", exact: true }),
    ).toHaveAttribute("max", "7");
  });
}

for (const quantity of [0, 3, 10]) {
  test(`draw input bounds and blur normalization for ${quantity} mainboard cards`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[0]!.cards = [context_card(context(2), 2)];
    state.deck.sections[1]!.cards = quantity
      ? [context_card(context(1), quantity)]
      : [];
    state.deck = with_summary(state.deck);
    await page.goto("/decks/1/edit");
    await page.getByRole("button", { name: "卡组统计", exact: true }).click();
    const area = page.locator("section").filter({
      has: page.getByRole("heading", { name: "抽到概率", exact: true }),
    });
    const least = area.getByRole("spinbutton", {
      name: "至少张数",
      exact: true,
    });
    const draws = area.getByRole("spinbutton", {
      name: "抽牌张数",
      exact: true,
    });
    await expect(least).toHaveValue(quantity ? "1" : "0");
    await expect(draws).toHaveValue(String(Math.min(7, quantity)));
    if (!quantity) {
      await expect(least).toBeDisabled();
      await expect(draws).toBeDisabled();
      for (const button of await area.getByRole("button").all())
        await expect(button).toBeDisabled();
      await area.getByRole("combobox", { name: "概率分组" }).click();
      await page.getByRole("option", { name: "产费颜色", exact: true }).click();
      await expect(area.locator("tbody tr td:last-child")).toHaveText(
        Array(7).fill("0.00%"),
      );
      return;
    }
    await least.fill("2");
    await draws.focus();
    await expect(draws).toHaveAttribute("min", "2");
    await draws.fill("1");
    await draws.blur();
    await expect(draws).toHaveValue("2");
    await expect(least).toHaveAttribute("max", "2");
    await least.fill("99");
    await least.blur();
    await expect(least).toHaveValue("2");
    await draws.fill("999");
    await draws.blur();
    await expect(draws).toHaveValue(String(quantity));
    await expect(
      area.getByRole("button", { name: "增加抽牌张数", exact: true }),
    ).toBeDisabled();
    await least.fill("1.9");
    await least.blur();
    await expect(least).toHaveValue("1");
    await draws.fill("");
    await expect(draws).toHaveValue("");
    await draws.blur();
    await expect(draws).toHaveValue("1");
    await least.fill("");
    await least.blur();
    await expect(least).toHaveValue("1");
    await least.fill("-5");
    await least.blur();
    await expect(least).toHaveValue("1");
    await expect(
      area.getByRole("button", { name: "减少抽牌张数", exact: true }),
    ).toBeDisabled();
    await expect(
      area.getByRole("button", { name: "增加至少张数", exact: true }),
    ).toBeDisabled();
  });
}

for (const removal of ["leave", "blank", "other", "same"] as const) {
  test(`zero-quantity cards remain until ${removal} ends their selection`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1]!.cards = [
      context_card(context(1), 1),
      context_card(context(2, { type_group: "artifact" }), 1),
      context_card(context(3), 1),
    ];
    await page.goto("/decks/1/edit");
    const first = page.getByRole("button", { name: /测试卡牌 1，\d 张/ }),
      minus = page.getByRole("button", { name: "减少测试卡牌 1", exact: true }),
      plus = page.getByRole("button", { name: "增加测试卡牌 1", exact: true });
    await first.hover({ position: { x: 70, y: 12 } });
    if (removal !== "leave") await first.click({ position: { x: 70, y: 12 } });
    await minus.click();
    await expect(first).toHaveAttribute("aria-label", "测试卡牌 1，0 张");
    await expect(minus).toBeDisabled();
    await plus.click();
    await expect(first).toHaveAttribute("aria-label", "测试卡牌 1，1 张");
    await minus.click();
    if (removal !== "leave") {
      await page.mouse.move(1, 1);
      await page.waitForTimeout(200);
      await expect(first).toBeVisible();
      await expect(page.getByRole("tooltip")).toHaveCount(0);
    }
    if (removal === "leave") await page.mouse.move(1, 1);
    else if (removal === "blank") await page.mouse.click(1, 1);
    else if (removal === "same") {
      await first.click({ position: { x: 70, y: 12 } });
      await expect(first).toBeVisible();
      await expect(
        page.locator('[data-deck-card][class*="expanded"]'),
      ).toHaveCount(1);
      await page.mouse.move(1, 1);
    } else
      await page
        .getByRole("button", { name: "测试卡牌 2，1 张", exact: true })
        .hover();
    await expect(first).toHaveCount(0);
    if (removal === "other") {
      await expect(page.getByRole("tooltip")).toContainText("测试卡牌 2");
      await page.mouse.move(1, 1);
    }
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await expect(
      page.locator('[data-deck-card][class*="expanded"]'),
    ).toHaveCount(0);
  });
}

for (const restore of [false, true]) {
  test(`saving a fixed zero-quantity cover normalizes its payload, restore=${restore}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1]!.cards = [context_card(context(1), 1)];
    await page.goto("/decks/1/edit");
    const card = page.getByRole("button", { name: /测试卡牌 1，\d 张/ });
    await card.click({ position: { x: 70, y: 12 } });
    await page
      .getByRole("button", { name: "减少测试卡牌 1", exact: true })
      .click();
    await expect(card).toHaveAttribute("aria-label", "测试卡牌 1，0 张");
    await expect(card).toHaveClass(/selected/);
    if (restore)
      await page
        .getByRole("button", { name: "增加测试卡牌 1", exact: true })
        .click();
    await page.getByRole("button", { name: "保存卡组", exact: true }).click();
    await expect(page).toHaveURL("/decks/1");
    expect(state.save_body?.cover_printing_uuid).toBe(restore ? uuid(1) : null);
    expect(
      state.save_body?.sections.flatMap((section) => section.cards),
    ).toHaveLength(restore ? 1 : 0);
  });
}

for (const outcome of ["success", "failure"] as const) {
  test(`pending additions block conflicting requests and recover after ${outcome}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    let release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    let context_started!: () => void;
    const started = new Promise<void>((resolve) => {
      context_started = resolve;
    });
    let conflicts = 0;
    page.on("request", (request) => {
      if (/\/(save|parse_code)$/.test(request.url())) conflicts++;
    });
    await page.route("**/api/v1/decks/1/card_contexts", async (route) => {
      context_started();
      await pending;
      await route.fulfill(
        outcome === "failure"
          ? {
              status: 503,
              json: {
                status: 503,
                data: null,
                error: {
                  code: "dependency_unavailable",
                  message: "unavailable",
                },
                request_id: "test",
              },
            }
          : {
              json: {
                status: 200,
                data: {
                  items: [{ ...context(2), requested_printing_uuid: uuid(2) }],
                },
                error: null,
                request_id: "test",
              },
            },
      );
    });
    await page.goto("/decks/1/edit");
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
    await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
    const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
    await modal.getByRole("button", { name: "搜索", exact: true }).click();
    await modal.getByRole("button", { name: "加入", exact: true }).click();
    await started;
    const adding = modal.getByRole("button", { name: "加入中", exact: true });
    await expect(adding).toBeDisabled();
    await expect(adding).toHaveCSS("opacity", "0.3");
    await expect(modal.getByRole("spinbutton")).toBeDisabled();
    await expect(modal.getByRole("spinbutton")).toHaveCSS("opacity", "0.3");
    await expect(
      modal.getByRole("button", { name: "×", exact: true }),
    ).toBeDisabled();
    await expect(modal.getByRole("textbox")).toBeEnabled();
    await modal.getByRole("button", { name: "搜索", exact: true }).click();
    await expect(adding).toBeDisabled();
    for (const name of ["保存卡组", "读取卡组配置"]) {
      const button = page.getByRole("button", {
        name,
        exact: true,
        includeHidden: true,
      });
      await expect(button).toBeEnabled();
      await button.evaluate((element: HTMLButtonElement) => element.click());
      await expect(button).toBeEnabled();
    }
    expect(conflicts).toBe(0);
    release();
    if (outcome === "failure") {
      const error = page.getByRole("dialog", { name: "操作失败", exact: true });
      await expect(error).toBeVisible();
      await error.getByRole("button", { name: "关闭", exact: true }).click();
    }
    await expect(
      modal.getByRole("button", { name: "加入", exact: true }),
    ).toBeEnabled();
    await expect(modal.getByRole("spinbutton")).toBeEnabled();
    await modal.getByRole("button", { name: "×", exact: true }).click();
    await page.getByRole("button", { name: "保存卡组", exact: true }).click();
    await expect(page).toHaveURL("/decks/1");
    const cards = state.save_body?.sections.flatMap((section) => section.cards);
    expect(cards?.some((card) => card.printing_uuid === uuid(2))).toBe(
      outcome === "success",
    );
  });

  test(`a closed editing session ignores a late context ${outcome}`, async ({
    page,
  }) => {
    await mock_api(page);
    let context_requests = 0;
    let release!: () => void;
    let started!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    const requested = new Promise<void>((resolve) => {
      started = resolve;
    });
    await page.route("**/api/v1/decks/1/card_contexts", async (route) => {
      const first = ++context_requests === 1;
      if (first) {
        started();
        await pending;
      }
      if (first && outcome === "failure") {
        await route.fulfill({
          status: 503,
          json: {
            status: 503,
            data: null,
            error: { code: "dependency_unavailable", message: "unavailable" },
            request_id: "test",
          },
        });
        return;
      }
      await route.fulfill({
        json: {
          status: 200,
          data: {
            items: [
              {
                ...context(2, { owned_quantity: first ? 2 : 0 }),
                requested_printing_uuid: uuid(2),
              },
            ],
          },
          error: null,
          request_id: "test",
        },
      });
    });

    async function request_add() {
      await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
      const modal = page.getByRole("dialog");
      await modal.getByRole("button", { name: "搜索", exact: true }).click();
      await modal.getByRole("button", { name: "加入", exact: true }).click();
      return modal;
    }

    await page.goto("/decks/1/edit");
    const old_modal = await request_add();
    await requested;
    await expect(
      old_modal.getByRole("button", { name: "×", exact: true }),
    ).toBeDisabled();
    page.once("dialog", (dialog) => dialog.accept());
    await page.reload();
    await expect(
      page.getByRole("textbox", { name: "卡组名称", exact: true }),
    ).toBeVisible();

    release();
    await expect
      .poll(() =>
        page.evaluate(async () => {
          const modulePath = "/src/api/query.ts";
          const { query_client } = await import(/* @vite-ignore */ modulePath);
          return query_client
            .getMutationCache()
            .getAll()
            .some(
              (mutation: { state: { status: string } }) =>
                mutation.state.status === "pending",
            );
        }),
      )
      .toBe(false);
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "测试卡牌 2，1 张", exact: true }),
    ).toHaveCount(0);

    const new_modal = await request_add();
    await expect(
      new_modal.getByRole("button", { name: "加入", exact: true }),
    ).toBeEnabled();
    await new_modal.getByRole("button", { name: "×", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "测试卡牌 2，1 张", exact: true }),
    ).toBeVisible();
    expect(context_requests).toBe(2);
    await expect(
      page.getByRole("button", { name: "数量不足", exact: true }),
    ).toBeVisible();
  });
}

test("reentering the editor refreshes contexts while one session reuses them", async ({
  page,
}) => {
  await mock_api(page);
  let owned = 2,
    context_requests = 0;
  await page.route("**/api/v1/decks/1/card_contexts", (route) => {
    context_requests++;
    return route.fulfill({
      json: {
        status: 200,
        data: {
          items: [
            {
              ...context(2, { owned_quantity: owned }),
              requested_printing_uuid: uuid(2),
            },
          ],
        },
        error: null,
        request_id: "test",
      },
    });
  });
  async function add(quantity: number) {
    await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
    const modal = page.getByRole("dialog");
    await modal.getByRole("button", { name: "搜索", exact: true }).click();
    await modal.getByRole("button", { name: "加入", exact: true }).click();
    await modal.getByRole("button", { name: "×", exact: true }).click();
    await expect(
      page.getByRole("button", { name: `测试卡牌 2，${quantity} 张` }),
    ).toBeVisible();
  }
  await page.goto("/decks/1/edit");
  await add(1);
  await add(2);
  expect(context_requests).toBe(1);
  await page.getByRole("link", { name: "卡组", exact: true }).click();
  await page.getByRole("button", { name: "放弃修改并离开" }).click();
  await expect(page).toHaveURL("/decks");
  owned = 0;
  await page.getByRole("link", { name: "查看测试卡组", exact: true }).click();
  await page.getByRole("link", { name: "编辑卡组", exact: true }).click();
  await add(1);
  expect(context_requests).toBe(2);
  await expect(
    page.getByRole("button", { name: "数量不足", exact: true }),
  ).toBeVisible();
});

for (const cached of [false, true]) {
  test(`editor entry loading waits for success and recovers on browser reload, cached=${cached}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    if (cached) {
      await page.goto("/decks/1");
      await expect(
        page.getByRole("heading", { name: state.deck.name, exact: true }),
      ).toBeVisible();
    }
    state.deck = with_summary({
      ...state.deck,
      name: "后端最新卡组",
      sections: state.deck.sections.map((section) => ({
        ...section,
        cards: section.cards.map((card) => ({ ...card, quantity: 5 })),
      })),
    });
    let fail = true,
      release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route(/\/api\/v1\/decks\/1$/, async (route) => {
      if (fail) {
        await route.fulfill({
          status: 503,
          json: {
            status: 503,
            data: null,
            error: { code: "dependency_unavailable", message: "unavailable" },
            request_id: "test",
          },
        });
      } else {
        await pending;
        await route.fallback();
      }
    });
    if (cached)
      await page.getByRole("link", { name: "编辑卡组", exact: true }).click();
    else await page.goto("/decks/1/edit");
    const error = page.getByRole("dialog", { name: "操作失败", exact: true }),
      name = page.getByRole("textbox", { name: "卡组名称", exact: true }),
      save = page.getByRole("button", { name: "保存卡组", exact: true });
    for (let attempt = 0; attempt < 2; attempt++) {
      await expect(error).toBeVisible();
      await error.getByRole("button", { name: "关闭", exact: true }).click();
      await expect(name).toHaveCount(0);
      await expect(save).toHaveCount(0);
      await expect(
        page.getByRole("button", { name: "重新加载卡组", exact: true }),
      ).toHaveCount(0);
      expect(state.save_body).toBeUndefined();
      if (attempt === 0) await page.reload();
    }
    fail = false;
    await page.reload();
    await expect(page.getByLabel("加载中", { exact: true })).toBeVisible();
    await expect(name).toHaveCount(0);
    await expect(save).toHaveCount(0);
    release();
    await expect(name).toHaveValue("后端最新卡组");
    await expect(
      page.getByRole("button", { name: "测试卡牌 1，5 张", exact: true }),
    ).toBeVisible();
    await save.click();
    await expect(page).toHaveURL("/decks/1");
    expect(state.save_body?.name).toBe("后端最新卡组");
    expect(
      state.save_body?.sections.find((section) => section.type === "mainboard")
        ?.cards[0]?.quantity,
    ).toBe(5);
  });
}

test("background detail refresh preserves the in-memory draft until reload", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/decks/1/edit");
  const name = page.getByRole("textbox", { name: "卡组名称", exact: true });
  const source = page.getByRole("textbox", { name: "卡组代码", exact: true });
  await page.getByRole("button", { name: "卡组代码", exact: false }).click();
  await name.fill("未保存名称");
  await source.fill("未读取的输入");
  await page.getByRole("button", { name: "测试卡牌 1，2 张" }).hover();
  await page
    .getByRole("button", { name: "增加测试卡牌 1", exact: true })
    .click();

  async function refresh() {
    await page.evaluate(async () => {
      const modulePath = "/src/api/query.ts";
      const { query_client } = await import(/* @vite-ignore */ modulePath);
      await query_client.invalidateQueries({
        queryKey: ["decks", "detail", "1"],
      });
    });
  }

  state.deck = { ...state.deck, name: "后端的新名称" };
  await refresh();
  await expect(name).toHaveValue("未保存名称");
  await expect(source).toHaveValue("未读取的输入");
  await expect(
    page.getByRole("button", { name: "测试卡牌 1，3 张" }),
  ).toBeVisible();

  await page.route(
    /\/api\/v1\/decks\/1$/,
    (route) =>
      route.fulfill({
        status: 503,
        json: {
          status: 503,
          data: null,
          error: { code: "dependency_unavailable", message: "unavailable" },
          request_id: "test",
        },
      }),
    { times: 1 },
  );
  await refresh();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(name).toHaveValue("未保存名称");
  await expect(source).toHaveValue("未读取的输入");
  await expect(
    page.getByRole("button", { name: "测试卡牌 1，3 张" }),
  ).toBeVisible();

  page.once("dialog", (dialog) => dialog.accept());
  await page.reload();
  await expect(name).toHaveValue("后端的新名称");
  await page.getByRole("button", { name: "卡组代码", exact: false }).click();
  await expect(source).toHaveValue(state.deck.source_text);
  await expect(
    page.getByRole("button", { name: "测试卡牌 1，2 张" }),
  ).toBeVisible();
});
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
  const state = await mock_api(page);
  for (const { fixed, held, quantity } of [
    { fixed: false, held: false, quantity: 2 },
    { fixed: false, held: true, quantity: 2 },
    { fixed: true, held: false, quantity: 2 },
    { fixed: true, held: true, quantity: 2 },
    { fixed: true, held: false, quantity: 1 },
  ]) {
    state.deck.sections[1]!.cards[0]!.quantity = quantity;
    await page.goto("/decks/1/edit");
    const source = page.getByRole("button", {
      name: `测试卡牌 1，${quantity} 张`,
    });
    await expect(source).toBeVisible();
    await page.evaluate(() => document.body.scrollTo({ top: 180 }));
    await source.hover({ position: { x: 70, y: 12 } });
    if (fixed) {
      await source.click({ position: { x: 70, y: 12 } });
      await page.mouse.move(1, 1);
    }
    await expect(
      page.locator('[data-deck-card][class*="expanded"]'),
    ).toHaveCount(1);
    const region = page.locator("section").filter({
      has: page.getByRole("heading", { name: "备牌", exact: true }),
    });
    const source_box = (await source.boundingBox())!,
      target = (await region.boundingBox())!;
    await page.mouse.move(source_box.x + 75, source_box.y + 90);
    await page.mouse.down();
    if (held) await page.waitForTimeout(400);
    await page.mouse.move(source_box.x + 81, source_box.y + 90);
    await page.mouse.move(target.x + 420, target.y + 24, { steps: 8 });
    await page.mouse.up();
    await expect(
      region.getByRole("button", {
        name: `测试卡牌 1，${held ? quantity : 1} 张`,
      }),
    ).toBeVisible();
    await expect(
      page.locator('[data-deck-card][class*="expanded"]'),
    ).toHaveCount(fixed && !held && quantity > 1 ? 1 : 0);
    await page.mouse.move(1, 1);
    await page.waitForTimeout(200);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    if (fixed && !held && quantity > 1) {
      const remaining = page
        .locator('[data-deck-card][class*="expanded"]')
        .getByRole("button", { name: "测试卡牌 1，1 张", exact: true });
      await expect(remaining).toBeVisible();
      await remaining.hover({ position: { x: 70, y: 12 } });
      await expect(page.getByRole("tooltip")).toContainText("测试卡牌 1");
      await page.getByRole("tooltip").getByLabel("卡牌说明").hover();
      await page.waitForTimeout(200);
      await expect(page.getByRole("tooltip")).toContainText("测试卡牌 1");
      await page.mouse.move(1, 1);
      expect(await page.getByRole("tooltip").count()).toBe(0);
      await expect(
        page.locator('[data-deck-card][class*="expanded"]'),
      ).toHaveCount(1);
      await expect(remaining).toBeVisible();
    }
    expect(
      await page
        .getByRole("textbox", { name: "卡组代码", exact: true })
        .count(),
    ).toBe(0);
  }
});
