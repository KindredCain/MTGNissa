import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { context, uuid } from "../src/test/fixtures";
import { context_card, with_summary } from "../src/decks/model";

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
      if (route.endsWith("/edit"))
        await page
          .getByRole("button", { name: "卡组代码", exact: false })
          .click();
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

test("editor keeps one expanded card and clears the previous fixed selection", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.deck.sections[1]!.cards.push(
    context_card(context(2, { type_group: "artifact" }), 1),
    context_card(context(3), 1),
  );
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "卡组代码", exact: false }).click();
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
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
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
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
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

test("background detail refresh preserves the in-memory draft until reload", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/decks/1/edit");
  const name = page.getByRole("textbox", { name: "卡组名称", exact: true });
  const source = page.getByRole("textbox", { name: "卡组代码", exact: true });
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
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
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
