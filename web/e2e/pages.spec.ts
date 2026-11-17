import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { context, detail, uuid } from "../src/test/fixtures";
import { context_card } from "../src/decks/model";

test("global search navigates before loading from any page", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/sets");
  for (const q of ["first", "second"]) {
    let release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route(`**/api/v1/search?q=${q}`, async (route) => {
      await pending;
      await route.fulfill({
        status: 503,
        json: {
          status: 503,
          data: null,
          error: { code: "dependency_unavailable", message: "Search failed" },
          request_id: "test",
        },
      });
    });
    const search = page.getByRole("textbox", { name: "搜索卡牌或输入语法…" });
    await search.fill(q);
    await search.press("Enter");
    try {
      await expect(page).toHaveURL(`/search?q=${q}`);
    } finally {
      release();
    }
    await expect(page.getByRole("dialog", { name: "操作失败" })).toBeVisible();
    await page.getByRole("button", { name: "关闭", exact: true }).click();
    await expect(page).toHaveURL(`/search?q=${q}`);
  }
});

for (const entry of ["language", "set"] as const) {
  for (const outcome of ["success", "failure", "leave"] as const) {
    test(`card ${entry} switch waits for data on ${outcome}`, async ({
      page,
    }) => {
      await mock_api(page);
      const initial = detail(),
        language = {
          ...initial.languages[0]!,
          printing_uuid: uuid(2),
          language: { code: "ja", display_name: "日文", sort_order: 2 },
          normal_quantity: 0,
        };
      initial.languages.push(language);
      initial.other_versions = [
        {
          set: { ...initial.card.set, code: "TST", display_name: "其他系列" },
          collector_number: "2",
          released_at: initial.card.released_at,
          normal_quantity: 0,
          foil_quantity: 0,
          languages: [language],
        },
      ];
      await page.route(`**/api/v1/cards/${uuid(1)}`, (route) =>
        route.fulfill({
          json: { status: 200, data: initial, error: null, request_id: "test" },
        }),
      );
      let release!: () => void;
      let started!: () => void;
      const pending = new Promise<void>((resolve) => {
          release = resolve;
        }),
        requested = new Promise<void>((resolve) => {
          started = resolve;
        });
      await page.route(`**/api/v1/cards/${uuid(2)}`, async (route) => {
        started();
        await pending;
        const next = detail();
        next.card.printing_uuid = uuid(2);
        next.faces[0]!.display_name = "目标版本";
        await route.fulfill({
          status: outcome === "failure" ? 503 : 200,
          json:
            outcome === "failure"
              ? {
                  status: 503,
                  data: null,
                  error: {
                    code: "dependency_unavailable",
                    message: "Version failed",
                  },
                  request_id: "test",
                }
              : { status: 200, data: next, error: null, request_id: "test" },
        });
      });
      await page.goto(`/cards/${uuid(1)}`);
      if (entry === "language")
        await page
          .getByRole("combobox", { name: "当前系列语言" })
          .selectOption(uuid(2));
      else await page.getByRole("link", { name: "日文", exact: true }).click();
      try {
        await requested;
        await expect(page).toHaveURL(`/cards/${uuid(1)}`);
        await expect(
          page.getByRole("heading", { name: "卡牌正面", exact: true }),
        ).toBeVisible();
        if (outcome === "leave") {
          await page
            .getByRole("link", { name: "操作日志", exact: true })
            .click();
          await expect(page).toHaveURL("/operation_logs");
        }
      } finally {
        release();
      }
      if (outcome === "success") {
        await expect(page).toHaveURL(`/cards/${uuid(2)}`);
        await expect(
          page.getByRole("heading", { name: "目标版本", exact: true }),
        ).toBeVisible();
      } else if (outcome === "failure") {
        await expect(
          page.getByRole("dialog", { name: "操作失败" }),
        ).toBeVisible();
        await page.getByRole("button", { name: "关闭", exact: true }).click();
        await expect(page).toHaveURL(`/cards/${uuid(1)}`);
        await expect(
          page.getByRole("heading", { name: "卡牌正面", exact: true }),
        ).toBeVisible();
      } else {
        await expect(
          page.getByRole("heading", { name: "操作日志", exact: true }),
        ).toBeVisible();
        await expect(page).toHaveURL("/operation_logs");
      }
    });
  }
}

for (const origin of ["/decks", "/decks/1"]) {
  test(`deleting from ${origin} navigates before loading the list`, async ({
    page,
  }) => {
    await mock_api(page);
    let deleted = false,
      release!: () => void,
      started!: () => void;
    const pending = new Promise<void>((resolve) => {
        release = resolve;
      }),
      requested = new Promise<void>((resolve) => {
        started = resolve;
      });
    const request_locations: { pathname: string; key: string }[] = [];
    await page.route("**/api/v1/decks**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/api/v1/decks/1/delete") deleted = true;
      if (
        path !== "/api/v1/decks" ||
        route.request().method() !== "GET" ||
        !deleted
      )
        return route.fallback();
      request_locations.push(
        await page.evaluate(() => ({
          pathname: window.location.pathname,
          key: window.history.state.key,
        })),
      );
      started();
      await pending;
      await route.fulfill({
        json: {
          status: 200,
          data: { items: [], total: 0, next_cursor: null },
          error: null,
          request_id: "test",
        },
      });
    });
    await page.goto(origin);
    const original_key = await page.evaluate(() => window.history.state.key);
    if (origin === "/decks")
      await page
        .getByRole("button", { name: "更多卡组操作", exact: true })
        .click();
    await page.getByRole("button", { name: "删除卡组", exact: true }).click();
    await page
      .getByRole("dialog", { name: "删除卡组？", exact: true })
      .getByRole("button", { name: "删除卡组", exact: true })
      .click();
    try {
      await requested;
      await expect(page).toHaveURL("/decks");
      expect(request_locations.length).toBeGreaterThan(0);
      for (const request_location of request_locations) {
        expect(request_location.pathname).toBe("/decks");
        expect(request_location.key).not.toBe(original_key);
      }
    } finally {
      release();
    }
    await expect(page.getByText("共 0 个卡组", { exact: true })).toBeVisible();
    await expect(page.locator("article")).toHaveCount(0);
  });
}

test("deck menus switch exclusively and close on repeated or outside clicks", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.route("**/api/v1/decks**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/v1/decks/2")
      return route.fulfill({
        json: {
          status: 200,
          data: { ...state.deck, deck_id: "2", name: "Deck B" },
          error: null,
          request_id: "test",
        },
      });
    if (path !== "/api/v1/decks" || route.request().method() !== "GET")
      return route.fallback();
    const item = {
      created_at: state.deck.created_at,
      updated_at: state.deck.updated_at,
      cover_card: null,
      section_counts: {
        commander: 0,
        mainboard: 2,
        sideboard: 0,
        maybeboard: 0,
      },
      counted_total: state.deck.summary.counted_total,
      color_distribution: { W: 0, U: 0, B: 0, R: 0, G: 2, C: 0 },
      exception: null,
    };
    return route.fulfill({
      json: {
        status: 200,
        data: {
          items: [
            { ...item, deck_id: "1", name: "Deck A" },
            { ...item, deck_id: "2", name: "Deck B" },
          ],
          total: 2,
          next_cursor: null,
        },
        error: null,
        request_id: "test",
      },
    });
  });
  await page.goto("/decks");
  const more = page.getByRole("button", { name: "更多卡组操作", exact: true }),
    edit = page.getByRole("button", { name: "编辑卡组", exact: true });
  await more.nth(0).click();
  await expect(edit).toHaveCount(1);
  await more.nth(1).click();
  await expect(more.nth(0)).toHaveAttribute("aria-expanded", "false");
  await expect(more.nth(1)).toHaveAttribute("aria-expanded", "true");
  await expect(edit).toHaveCount(1);
  await more.nth(1).click();
  await expect(edit).toHaveCount(0);
  await expect(page).toHaveURL("/decks");
  await more.nth(0).click();
  await page.mouse.click(1, 1);
  await expect(edit).toHaveCount(0);
  await expect(page).toHaveURL("/decks");
  await more.nth(0).click();
  await more.nth(1).click();
  await page.getByRole("button", { name: "删除卡组", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page).toHaveURL("/decks");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  for (const index of [0, 1]) {
    await more.nth(0).click();
    await page
      .locator("article")
      .nth(index)
      .click({ position: { x: 80, y: 50 } });
    await expect(page).toHaveURL(`/decks/${index + 1}`);
    await page.goto("/decks");
  }
});

test("series navigation, local filters and desktop card columns", async ({
  page,
}, info) => {
  await page.clock.install();
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
  await page.clock.fastForward(31_000);
  await page.getByRole("button", { name: "法术力", exact: true }).click();
  await page.getByRole("checkbox", { name: "1", exact: true }).click();
  await page.getByRole("heading", { name: "全部卡牌" }).click();
  await expect(page).toHaveURL(/mana_value=1/);
  await expect(page.locator("article")).toHaveCount(6);
  await page
    .getByRole("combobox", { name: "排序" })
    .selectOption("collector_number_desc");
  await expect(
    page.locator("article").first().getByRole("link"),
  ).toHaveAttribute("aria-label", "查看测试卡牌 41");
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
  const usage = page.locator("aside > section").last();
  await expect(
    usage.getByRole("heading", { name: "卡组使用情况", exact: true }),
  ).toBeVisible();
  await expect(
    usage.getByRole("link", { name: "测试卡组 ↗ 2" }),
  ).toHaveAttribute("href", "/decks/1");
  await page.getByRole("button", { name: "功能标签", exact: true }).click();
  await page.getByRole("link", { name: 'otag:"mana rock"' }).click();
  await expect(page).toHaveURL(/\/search\?q=/);
  expect(new URL(page.url()).searchParams.get("q")).toBe('otag:"mana rock"');
  expect(state.cards_requests).toBe(0);
});
for (const owned of [0, 6]) {
  test(`card usage summarizes all decks against ownership ${owned}`, async ({
    page,
  }) => {
    await mock_api(page);
    const data = detail();
    data.collection = { normal_quantity: owned, foil_quantity: 0 };
    data.logical_collection = {
      normal_quantity: owned,
      foil_quantity: 0,
      total_quantity: owned,
    };
    data.deck_usage = {
      total_decks: 2,
      decks: [
        { deck_id: "1", name: "Deck A", quantity: 2, sections: ["mainboard"] },
        { deck_id: "2", name: "Deck B", quantity: 2, sections: ["sideboard"] },
      ],
    };
    await page.route(`**/api/v1/cards/${uuid(1)}`, (route) =>
      route.fulfill({
        json: { status: 200, data, error: null, request_id: "test" },
      }),
    );
    await page.goto(`/cards/${uuid(1)}`);
    await expect(page.getByText(`${owned}:0*`, { exact: true })).toBeVisible();
    const summary = page.getByText(
      `出现在 2 个卡组 · 总需求 4 · 跨卡组缺口 ${Math.max(0, 4 - owned)}`,
      { exact: true },
    );
    await summary.scrollIntoViewIfNeeded();
    await expect(summary).toBeInViewport();
    await expect(
      page.getByRole("link", { name: "Deck B ↗ 2" }),
    ).toHaveAttribute("href", "/decks/2");
  });
}
for (const layout of ["adventure", "split", "flip"]) {
  test(`shared-image ${layout} details display every face without a flip button`, async ({
    page,
  }) => {
    await mock_api(page);
    const data = detail();
    data.card.layout = layout;
    data.card.images.back_url = null;
    data.card.images.has_distinct_back = false;
    data.faces.forEach((face, index) => {
      face.image_url = data.card.images.front_url;
      face.display_oracle_text = `Rules for face ${index}`;
    });
    data.faces[1]!.display_flavor_text = "Second face flavor";
    await page.route(`**/api/v1/cards/${uuid(1)}`, (route) =>
      route.fulfill({
        json: { status: 200, data, error: null, request_id: "test" },
      }),
    );
    await page.goto(`/cards/${uuid(1)}`);
    await expect(page.getByRole("heading", { name: "卡牌正面" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "卡牌背面" })).toBeVisible();
    await expect(
      page.getByText("Rules for face 0", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Rules for face 1", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Second face flavor", { exact: true }),
    ).toBeVisible();
    const second_face = page
      .locator("article")
      .filter({ hasText: "Rules for face 1" });
    await expect(second_face.getByText("Back", { exact: true })).toBeVisible();
    await expect(
      second_face.getByRole("img", { name: "{R}", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /查看[正反]面/ }),
    ).toHaveCount(0);
  });
}
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
for (const view of ["merge", "search"] as const) {
  test(`collection refreshes cached ${view} views without reloading the edited list`, async ({
    page,
  }) => {
    await mock_api(page);
    let collection_requests = 0;
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === "/api/v1/collection")
        collection_requests++;
    });
    await page.goto(
      view === "merge"
        ? "/collection"
        : "/collection?merge_languages=false&q=Test",
    );
    await expect(page.locator("article")).toHaveCount(1);
    const merge = page.getByRole("switch", {
        name: "合并不同语言",
        exact: true,
      }),
      search = page.getByRole("textbox", { name: "按卡名搜索我的卡牌" });
    if (view === "merge") {
      await merge.click();
      await expect(merge).not.toBeChecked();
    } else {
      await search.fill("");
      await search.press("Enter");
      await expect(page).toHaveURL(/merge_languages=false$/);
    }
    const before_adjustment = collection_requests;
    await page.getByRole("button", { name: "增加普通卡", exact: true }).click();
    await expect(page.locator("output").first()).toHaveText("3");
    expect(collection_requests).toBe(before_adjustment);
    if (view === "merge") {
      await merge.click();
      await expect(merge).toBeChecked();
      await expect(page.locator("article").first()).toContainText("普通 3");
    } else {
      await search.fill("Test");
      await search.press("Enter");
      await expect(page).toHaveURL(/q=Test/);
      await expect(page.locator("output").first()).toHaveText("3");
    }
    expect(collection_requests).toBe(before_adjustment + 1);
  });
}
for (const kind of ["collection", "wishlist"] as const) {
  test(`${kind} series headers collapse locally and expand with the keyboard`, async ({
    page,
  }) => {
    await mock_api(page);
    let list_requests = 0;
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === `/api/v1/${kind}`)
        list_requests++;
    });
    await page.goto(`/${kind}`);
    const header = page.getByRole("button", { name: /^KLD 卡拉德许/ }),
      card = page.locator("article").first();
    await expect(card).toBeVisible();
    await expect(header).toHaveAttribute("aria-expanded", "true");
    if (kind === "collection")
      await expect(header).toContainText("共 2 张，1 种");
    else await expect(header).not.toContainText("共");
    const requests = list_requests;
    await header.click({ position: { x: 500, y: 17 } });
    await expect(header).toHaveAttribute("aria-expanded", "false");
    await expect(card).toBeHidden();
    await expect(header).toBeVisible();
    await header.focus();
    await header.press("Enter");
    await expect(header).toHaveAttribute("aria-expanded", "true");
    await expect(card).toBeVisible();
    expect(list_requests).toBe(requests);
  });
}
test("wishlist cancellation retains its card and can be restored", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/wishlist");
  const name = page.locator("article").getByText("测试卡牌 1", { exact: true }),
    footer = name.locator("..");
  await expect(name).toBeVisible();
  await name.click();
  await footer.click({ position: { x: 120, y: 12 } });
  expect(state.included).toBe(true);
  await expect(page).toHaveURL("/wishlist");
  const footer_box = (await footer.boundingBox())!;
  expect(footer_box.width).toBe(228);
  expect(footer_box.height).toBe(24);
  await page.getByRole("button", { name: "移出愿望单" }).click();
  await expect(page.getByRole("button", { name: "恢复愿望单" })).toBeVisible();
  expect(state.included).toBe(false);
  await expect(page.locator("article")).toHaveCount(1);
  await page.getByRole("button", { name: "恢复愿望单" }).click();
  await expect(page.getByRole("button", { name: "移出愿望单" })).toBeVisible();
  await page.getByRole("button", { name: "移出愿望单" }).click();
  await expect(page.getByRole("button", { name: "恢复愿望单" })).toBeVisible();
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
  await expect(page.getByRole("dialog")).toContainText("离开卡组编辑？");
  await page.getByRole("button", { name: "继续编辑" }).click();
  await page.getByRole("button", { name: "保存卡组", exact: true }).click();
  await expect(page).toHaveURL("/decks/1");
  expect(state.save_body?.name).toBe("未保存名称");
  expect(JSON.stringify(state.save_body)).not.toContain("source_text");
});
for (const route of ["/decks/1", "/decks/1/edit"]) {
  test(`deck image and translation follow the same face at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page),
      data = detail(),
      card = state.deck.sections[1]!.cards[0]!;
    card.card.images = data.card.images;
    card.faces = data.faces;
    card.faces[0]!.display_oracle_text = "Front face rules";
    card.faces[1]!.display_type_line = "地";
    card.faces[1]!.display_oracle_text = "Back face rules";
    await page.goto(route);
    const target = page.getByRole("button", {
        name: "测试卡牌 1，2 张",
        exact: true,
      }),
      flip = page.getByRole("button", {
        name: "翻面：测试卡牌 1",
        exact: true,
      }),
      preview = page.locator('[role="tooltip"]');
    await target.hover();
    await expect(
      preview.getByRole("heading", { name: "卡牌正面" }),
    ).toBeVisible();
    await expect(preview).toContainText("Front face rules");
    await expect(preview).not.toContainText("Back face rules");
    await flip.click();
    await expect(target.locator("img")).toHaveAttribute(
      "src",
      data.card.images.back_url!,
    );
    await expect(
      preview.getByRole("heading", { name: "卡牌背面" }),
    ).toBeVisible();
    await expect(preview).toContainText("地");
    await expect(preview).toContainText("Back face rules");
    await expect(preview).not.toContainText("Front face rules");
    if (route.endsWith("/edit")) {
      await target.click();
      await page.mouse.move(0, 0);
      await expect(preview).toHaveCount(0);
    } else {
      await page.mouse.move(0, 0);
    }
    await target.hover();
    await expect(preview).toContainText("Back face rules");
    await flip.click();
    await expect(target.locator("img")).toHaveAttribute(
      "src",
      data.card.images.front_url!,
    );
    await expect(preview).toContainText("Front face rules");
    await page.getByRole("link", { name: "卡组", exact: true }).click();
    await expect(page).toHaveURL("/decks");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test(`shared-image deck previews include every face at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page),
      card = state.deck.sections[1]!.cards[0]!;
    card.faces = detail().faces;
    card.faces[0]!.display_oracle_text = "First shared face rules";
    card.faces[1]!.display_oracle_text = "Second shared face rules";
    card.card.layout = "split";
    await page.goto(route);
    await page
      .getByRole("button", { name: "测试卡牌 1，2 张", exact: true })
      .hover();
    const preview = page.locator('[role="tooltip"]');
    await expect(preview).toContainText("First shared face rules");
    await expect(preview).toContainText("Second shared face rules");
    await expect(
      page.getByRole("button", { name: "翻面：测试卡牌 1", exact: true }),
    ).toHaveCount(0);
  });

  test(`deck text preview stays complete without card images at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    const rules = Array.from(
      { length: 8 },
      (_, index) => `Rule line ${index}`,
    ).join("\n");
    state.deck.sections[1]!.cards[0]!.faces[0]!.display_oracle_text = rules;
    state.deck.sections[1]!.cards.push(context_card(context(2), 1));
    await page.goto(route);
    const target = page.getByRole("button", {
        name: "测试卡牌 1，2 张",
        exact: true,
      }),
      slot = target.locator("..");
    await target.hover({ position: { x: 70, y: 12 } });
    const preview = page.locator('[role="tooltip"]');
    await expect(preview).toBeVisible();
    await expect(
      preview.getByRole("heading", { name: "测试卡牌 1" }),
    ).toBeVisible();
    await expect(preview.getByText("类别", { exact: true })).toBeVisible();
    await expect(preview.getByText("规则文字", { exact: true })).toBeVisible();
    await expect(preview.locator("img")).toHaveCount(0);
    await expect(preview.locator("p").last()).toHaveText(rules);
    const geometry = await preview.evaluate((element) => {
      const rect = element.getBoundingClientRect();
      return {
        width: rect.width,
        height: rect.height,
        clipped: element.scrollHeight > element.clientHeight,
        background: getComputedStyle(element).backgroundColor,
      };
    });
    expect(geometry.width).toBe(300);
    expect(geometry.height).toBeGreaterThan(200);
    expect(geometry.clipped).toBe(false);
    expect(geometry.background).toBe("rgb(255, 255, 255)");
    await preview.getByLabel("卡牌说明").hover();
    await page.waitForTimeout(200);
    await expect(preview).toBeVisible();
    await expect(slot).toHaveClass(/expanded/);
    await page.mouse.move(1, 1);
    expect(await preview.count()).toBe(0);
    await expect(slot).not.toHaveClass(/expanded/);
    await target.hover({ position: { x: 70, y: 12 } });
    await expect(preview).toBeVisible();
    await expect(slot).toHaveClass(/expanded/);
    await page.mouse.move(1, 1);
    await expect(preview).toHaveCount(0);
    await expect(slot).not.toHaveClass(/expanded/);
    if (!route.endsWith("/edit")) {
      await target.click({ position: { x: 70, y: 12 } });
      await expect(page).toHaveURL(`/cards/${uuid(1)}`);
    }
  });
}
test("add-card previews read the front face from the new candidate contract", async ({
  page,
}) => {
  await mock_api(page);
  const candidate = context(2),
    data = detail();
  candidate.card.images = data.card.images;
  await page.route("**/api/v1/decks/1/cards/search**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        data: {
          items: [{ card: candidate.card, faces: data.faces }],
          total: 1,
          next_cursor: null,
        },
        error: null,
        request_id: "test",
      },
    }),
  );
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "搜索", exact: true })
    .click();
  await page.getByRole("button", { name: "测试卡牌 2", exact: true }).hover();
  const preview = page.locator('[role="tooltip"]');
  await expect(
    preview.getByRole("heading", { name: "卡牌正面" }),
  ).toBeVisible();
  await expect(preview.getByRole("heading", { name: "卡牌背面" })).toHaveCount(
    0,
  );
  await expect(preview.locator("img")).toHaveAttribute(
    "src",
    data.card.images.front_url!,
  );
  await expect(preview.getByRole("button")).toHaveCount(0);
});

for (const kind of ["detail", "edit", "add"] as const) {
  test(`${kind} preview limits height and scrolls text without moving the page`, async ({
    page,
  }) => {
    const state = await mock_api(page),
      rules = Array.from({ length: 90 }, (_, i) => `Rule line ${i}`).join("\n");
    state.deck.sections[1]!.cards[0]!.faces[0]!.display_oracle_text = rules;
    if (kind === "add") {
      const candidate = context(2);
      candidate.faces[0]!.display_oracle_text = rules;
      await page.route("**/api/v1/decks/1/cards/search**", (route) =>
        route.fulfill({
          json: {
            status: 200,
            data: {
              items: [{ card: candidate.card, faces: candidate.faces }],
              total: 1,
              next_cursor: null,
            },
            error: null,
            request_id: "test",
          },
        }),
      );
    }
    await page.goto(kind === "detail" ? "/decks/1" : "/decks/1/edit");
    if (kind === "add") {
      await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "搜索", exact: true })
        .click();
      await page
        .getByRole("button", { name: "测试卡牌 2", exact: true })
        .hover();
    } else {
      await page
        .getByRole("button", { name: "测试卡牌 1，2 张", exact: true })
        .hover();
    }
    const preview = page.locator('[role="tooltip"]'),
      text = preview.locator('[tabindex="0"]');
    await expect(preview).toBeVisible();
    const box = (await preview.boundingBox())!;
    expect(box.height).toBeLessThanOrEqual(kind === "add" ? 480 : 360);
    expect(box.y).toBeGreaterThanOrEqual(7.99);
    expect(box.y + box.height).toBeLessThanOrEqual(
      page.viewportSize()!.height - 7.99,
    );
    const start = await page.evaluate(() => ({
      x: scrollX,
      y: scrollY,
      tables: [...document.querySelectorAll('[role="dialog"] > div')].map(
        (el) => el.scrollTop,
      ),
    }));
    await text.hover();
    await page.waitForTimeout(200);
    await expect(preview).toBeVisible();
    await page.mouse.wheel(0, 350);
    await expect
      .poll(() => text.evaluate((el) => el.scrollTop))
      .toBeGreaterThan(0);
    await expect(text).toContainText("Rule line 89");
    await text.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    await page.mouse.wheel(0, 1000);
    await expect(preview).toBeVisible();
    expect(
      await page.evaluate(() => ({
        x: scrollX,
        y: scrollY,
        tables: [...document.querySelectorAll('[role="dialog"] > div')].map(
          (el) => el.scrollTop,
        ),
      })),
    ).toEqual(start);
    await page.mouse.move(0, 0);
    expect(await preview.count()).toBe(0);
  });
}

test("add-card overlay and statistics keep background locked", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  const modal = page.getByRole("dialog");
  await expect(modal).toBeVisible();
  const geometry = await modal.evaluate((element) => {
    const rect = element.getBoundingClientRect(),
      input = element.querySelector("input")!,
      input_rect = input.getBoundingClientRect();
    return {
      width: rect.width,
      height: rect.height,
      left: rect.left,
      top: rect.top,
      layout_width: (element as HTMLElement).offsetWidth,
      layout_height: (element as HTMLElement).offsetHeight,
      input_scale: input_rect.width / input.offsetWidth,
      viewport_width: window.visualViewport?.width ?? innerWidth,
      viewport_height: window.visualViewport?.height ?? innerHeight,
    };
  });
  const height = Math.min(702, geometry.viewport_height - 64);
  expect(geometry.layout_width).toBe(940);
  expect(geometry.layout_height).toBe(height);
  expect(geometry.width).toBeCloseTo(940, 2);
  expect(geometry.height).toBeCloseTo(height, 2);
  expect(geometry.input_scale).toBeCloseTo(1, 3);
  expect(geometry.left).toBeCloseTo(
    (geometry.viewport_width - geometry.width) / 2,
    2,
  );
  expect(geometry.top).toBeCloseTo(
    (geometry.viewport_height - geometry.height) / 2,
    2,
  );
  expect(geometry.top).toBeGreaterThanOrEqual(31.99);
  await expect(page.locator("body")).toHaveAttribute("data-scroll-locked", "1");
  await modal.getByRole("button", { name: "搜索", exact: true }).click();
  await expect(
    modal.getByRole("button", { name: "加入", exact: true }),
  ).toBeVisible();
  await modal.getByRole("button", { name: "测试卡牌 2", exact: true }).hover();
  const image_preview = page.locator('[role="tooltip"]');
  await expect(image_preview).toBeVisible();
  await expect(image_preview.locator('img[alt="测试卡牌 2"]')).toBeVisible();
  expect((await image_preview.boundingBox())!.width).toBe(440);
  const layer = await image_preview.evaluate((element) => ({
    preview: Number(getComputedStyle(element).zIndex),
    dialog: Number(
      getComputedStyle(document.querySelector('[role="dialog"]')!).zIndex,
    ),
    pointer_events: getComputedStyle(element).pointerEvents,
  }));
  expect(layer.preview).toBeGreaterThan(layer.dialog);
  expect(layer.pointer_events).toBe("auto");
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
test("log date inputs follow history and only submit on query", async ({
  page,
}) => {
  await mock_api(page);
  const requests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/v1/operation_logs/summary")
      requests.push(request.url());
  });
  await page.goto("/operation_logs");
  const start = page.getByLabel("开始日期"),
    end = page.getByLabel("结束日期");
  await expect(page.locator("tbody tr")).not.toHaveCount(0);
  const initial_requests = requests.length;
  await start.fill("2026-10-01");
  await end.fill("2026-10-31");
  expect(new URL(page.url()).search).toBe("");
  expect(requests).toHaveLength(initial_requests);
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await expect(page).toHaveURL(/start_date=2026-10-01.*end_date=2026-10-31/);
  expect(new URL(requests.at(-1)!).searchParams.get("start_date")).toBe(
    "2026-10-01",
  );
  expect(new URL(requests.at(-1)!).searchParams.get("end_date")).toBe(
    "2026-10-31",
  );
  await page.goBack();
  await expect(page).toHaveURL(/\/operation_logs$/);
  await expect(start).toHaveValue("");
  await expect(end).toHaveValue("");
  await page.goForward();
  await expect(start).toHaveValue("2026-10-01");
  await expect(end).toHaveValue("2026-10-31");
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
