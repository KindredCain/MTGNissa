import { test, expect, type Request } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, card_face, context, detail, uuid } from "../src/test/fixtures";
import { context_card, with_summary } from "../src/decks/model";

for (const route of [
  "/sets/KLD/cards?language=en",
  "/search?q=card",
  "/collection",
  "/wishlist",
]) {
  test(`ordinary list previews keep the same bubble and reuse cached text at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    await page.goto(route);
    const target = page.getByRole("link", {
      name: "查看测试卡牌 1",
      exact: true,
    });
    await expect(target).toBeVisible();
    expect(state.preview_requests).toBe(0);
    await target.hover();
    const preview = page.getByRole("tooltip");
    await expect(preview).toContainText("测试规则文字。");
    const initial_requests = state.preview_requests;
    expect(initial_requests).toBeGreaterThan(0);
    await preview.evaluate((element) =>
      element.setAttribute("data-roundtrip", "retained"),
    );
    await preview.getByLabel("卡牌说明").hover();
    await target.hover();
    await expect(preview).toHaveAttribute("data-roundtrip", "retained");
    await page.mouse.move(1, 1);
    await expect(preview).toHaveCount(0);
    await target.focus();
    await expect(preview).toBeVisible();
    expect(state.preview_requests).toBe(initial_requests);
    await page.keyboard.press("Escape");
    await expect(preview).toHaveCount(0);
  });
}

test("list preview delays the first request for 300ms and expires after five minutes", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  const target = page.getByRole("link", {
    name: "查看测试卡牌 1",
    exact: true,
  });
  await expect(target).toBeVisible();
  const now = new Date();
  await page.clock.install({ time: now });
  await page.clock.pauseAt(new Date(now.getTime() + 1000));
  await target.hover();
  await page.clock.runFor(299);
  expect(state.preview_requests).toBe(0);
  await page.clock.runFor(1);
  const preview = page.getByRole("tooltip");
  await expect(preview).toBeVisible();
  const initial_requests = state.preview_requests;
  expect(initial_requests).toBeGreaterThan(0);
  await page.mouse.move(1, 1);
  await page.clock.runFor(151);
  await expect(preview).toHaveCount(0);
  await page.clock.runFor(4 * 60_000);
  await target.hover();
  await page.clock.runFor(301);
  await expect(preview).toBeVisible();
  expect(state.preview_requests).toBe(initial_requests);
  await page.mouse.move(1, 1);
  await page.clock.runFor(151);
  await expect(preview).toHaveCount(0);
  await page.clock.runFor(61_000);
  await target.hover();
  await page.clock.runFor(301);
  await expect(preview).toBeVisible();
  await expect
    .poll(() => state.preview_requests)
    .toBeGreaterThan(initial_requests);
});

test("leaving a list card before the delay does not request its preview", async ({
  page,
}) => {
  const state = await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  const target = page.getByRole("link", {
    name: "查看测试卡牌 1",
    exact: true,
  });
  await expect(target).toBeVisible();
  const now = new Date();
  await page.clock.install({ time: now });
  await page.clock.pauseAt(new Date(now.getTime() + 1000));
  await target.hover();
  await page.clock.runFor(150);
  await page.mouse.move(1, 1);
  await page.clock.runFor(500);
  expect(state.preview_requests).toBe(0);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
});

test("list previews reuse detail cache and switch every face without another request", async ({
  page,
}) => {
  const state = await mock_api(page);
  const data = detail();
  await page.route("**/api/v1/cards?**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "faces",
        data: {
          set: data.card.set,
          language: data.card.language,
          items: [data.card],
          total: 1,
        },
      },
    }),
  );
  await page.goto("/sets/KLD/cards?language=en");
  await page.evaluate(async (data) => {
    const path = "/src/api/query.ts";
    const { query_client } = await import(/* @vite-ignore */ path);
    query_client.setQueryData(["card", data.card.printing_uuid], data);
  }, data);
  await page
    .getByRole("link", { name: `查看${data.card.display_name}`, exact: true })
    .hover();
  const preview = page.getByRole("tooltip");
  await expect(
    preview.getByRole("heading", { name: "卡牌正面" }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: `翻面：${data.card.display_name}`,
      exact: true,
    })
    .click();
  await expect(
    preview.getByRole("heading", { name: "卡牌背面" }),
  ).toBeVisible();
  expect(state.preview_requests).toBe(0);
});

test("list preview stays below sticky headers and hides a covered focused card", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  const target = page.getByRole("link", {
    name: "查看测试卡牌 1",
    exact: true,
  });
  await target.focus();
  const preview = page.getByRole("tooltip");
  await expect(preview).toBeVisible();
  await page.locator("body").evaluate((element) => {
    element.scrollTop = 180;
  });
  await expect
    .poll(async () =>
      preview.evaluate((element) => {
        const headers = Array.from(
          document.querySelectorAll("[data-preview-header], [data-toolbar]"),
          (header) => header.getBoundingClientRect(),
        );
        return (
          element.getBoundingClientRect().top >=
          Math.max(...headers.map((header) => header.bottom))
        );
      }),
    )
    .toBe(true);
  await page.locator("body").evaluate((element) => {
    element.scrollTop = 650;
  });
  await expect(preview).toHaveCount(0);
});

test("failed list preview uses the shared error dialog", async ({ page }) => {
  await mock_api(page);
  await page.route("**/api/v1/cards/*/preview", (route) =>
    route.fulfill({
      status: 503,
      json: {
        status: 503,
        data: null,
        error: { code: "dependency_unavailable", message: "unavailable" },
        request_id: "preview-failure",
      },
    }),
  );
  await page.goto("/sets/KLD/cards?language=en");
  await page.getByRole("link", { name: "查看测试卡牌 1", exact: true }).hover();
  await expect(
    page.getByRole("dialog", { name: "操作失败", exact: true }),
  ).toBeVisible();
});

test("Token cards never request a card preview", async ({ page }) => {
  const state = await mock_api(page);
  await page.goto("/sets/TKLD/tokens");
  await page.locator("main article").first().hover();
  await page.clock.install();
  await page.clock.runFor(500);
  expect(state.preview_requests).toBe(0);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
});

for (const layout of ["adventure", "split", "flip"]) {
  test(`list previews include every shared-image ${layout} face`, async ({
    page,
  }) => {
    await mock_api(page);
    const current = card(1, { layout });
    await page.route("**/api/v1/cards?**", (route) =>
      route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "shared-faces",
          data: {
            set: current.set,
            language: current.language,
            items: [current],
            total: 1,
          },
        },
      }),
    );
    await page.route(`**/api/v1/cards/${uuid(1)}/preview`, (route) =>
      route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "shared-faces",
          data: {
            faces: [
              card_face(1, { face_index: 0 }),
              card_face(2, { face_index: 1 }),
            ],
          },
        },
      }),
    );
    await page.goto("/sets/KLD/cards?language=en");
    await page
      .getByRole("link", { name: "查看测试卡牌 1", exact: true })
      .hover();
    const preview = page.getByRole("tooltip");
    await expect(
      preview.getByRole("heading", { name: "测试卡牌 1" }),
    ).toBeVisible();
    await expect(
      preview.getByRole("heading", { name: "测试卡牌 2" }),
    ).toBeVisible();
  });
}

test("leaving a loading card preview cancels its request", async ({ page }) => {
  await mock_api(page);
  let release!: () => void, last_request: Request | undefined;
  const aborted = new Set<Request>();
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  page.on("request", (request) => {
    if (request.url().endsWith(`/${uuid(1)}/preview`)) last_request = request;
  });
  page.on("requestfailed", (request) => aborted.add(request));
  await page.route(`**/api/v1/cards/${uuid(1)}/preview`, async (route) => {
    await pending;
    await route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "cancelled",
        data: { faces: [card_face(1)] },
      },
    });
  });
  await page.goto("/sets/KLD/cards?language=en");
  await page.getByRole("link", { name: "查看测试卡牌 1", exact: true }).hover();
  try {
    await expect(page.getByRole("status")).toHaveText("正在加载说明…");
    await page.mouse.move(1, 1);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await expect
      .poll(() => !!last_request && aborted.has(last_request))
      .toBe(true);
    await expect(page.getByRole("dialog", { name: "操作失败" })).toHaveCount(0);
  } finally {
    release();
  }
});

for (const route of ["/decks/1", "/decks/1/edit"]) {
  for (const kind of ["description", "shortage", "deck_usage"] as const) {
    test(`${kind} bubble keeps the same element during a protected roundtrip at ${route}`, async ({
      page,
    }) => {
      const state = await mock_api(page);
      if (kind !== "description") {
        const current = state.deck.sections[1].cards[0];
        current.owned_quantity = kind === "shortage" ? 0 : 2;
        current.other_decks_quantity = kind === "shortage" ? 0 : 3;
        current.other_decks_count = kind === "shortage" ? 0 : 1;
        state.deck = with_summary(state.deck);
      }
      await page.goto(route);
      const target = page.getByRole("button", {
        name:
          kind === "description"
            ? "测试卡牌 1，2 张"
            : kind === "shortage"
              ? "数量不足"
              : "卡组占用",
        exact: true,
      });
      await target.hover();
      const bubble =
        kind === "description"
          ? page.getByRole("tooltip")
          : page.locator("[data-exception-side]");
      await expect(bubble).toBeVisible();
      await bubble.evaluate((element) =>
        element.setAttribute("data-roundtrip", "retained"),
      );
      await bubble.hover();
      await expect(bubble).toHaveAttribute("data-roundtrip", "retained");
      const now = new Date();
      await page.clock.install({ time: now });
      await page.clock.pauseAt(new Date(now.getTime() + 1000));
      await page.mouse.move(1, 1);
      await page.clock.runFor(149);
      await expect(bubble).toHaveAttribute("data-roundtrip", "retained");
      await target.hover();
      await page.clock.runFor(200);
      await expect(bubble).toHaveAttribute("data-roundtrip", "retained");
      await page.mouse.move(1, 1);
      await page.clock.runFor(151);
      await expect(bubble).toHaveCount(0);
    });

    test(`${kind} bubble avoids headers and hides its covered anchor at ${route}`, async ({
      page,
    }) => {
      const state = await mock_api(page);
      if (kind !== "description") {
        const current = state.deck.sections[1].cards[0];
        current.owned_quantity = kind === "shortage" ? 0 : 2;
        current.other_decks_quantity = kind === "shortage" ? 0 : 3;
        current.other_decks_count = kind === "shortage" ? 0 : 1;
        state.deck = with_summary(state.deck);
      }
      await page.goto(route);
      const target = page.getByRole("button", {
        name:
          kind === "description"
            ? "测试卡牌 1，2 张"
            : kind === "shortage"
              ? "数量不足"
              : "卡组占用",
        exact: true,
      });
      await target.focus();
      const bubble =
        kind === "description"
          ? page.getByRole("tooltip")
          : page.locator("[data-exception-side]");
      await expect(bubble).toBeVisible();
      await page.locator("main").evaluate((element) => {
        element.style.minHeight = "2400px";
      });
      await target.evaluate((element) => {
        document.body.scrollTop += element.getBoundingClientRect().top - 65;
      });
      await expect
        .poll(async () =>
          bubble.evaluate((element) => {
            const headers = Array.from(
              document.querySelectorAll("[data-preview-header]"),
              (header) => header.getBoundingClientRect(),
            );
            return (
              element.getBoundingClientRect().top >=
              Math.max(...headers.map((header) => header.bottom))
            );
          }),
        )
        .toBe(true);
      await page.locator("body").evaluate((element) => {
        element.scrollTop += window.innerHeight;
      });
      await expect
        .poll(async () => {
          const box = (await target.boundingBox())!;
          return box.y + box.height;
        })
        .toBeLessThanOrEqual(0);
      await expect(bubble).toHaveCount(0);
    });
  }
}

for (const route of ["/decks/1", "/decks/1/edit"]) {
  test(`card description follows page scrolling at ${route}`, async ({
    page,
  }) => {
    await mock_api(page);
    await page.goto(route);
    const target = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await expect(target).toBeVisible();
    if (route.endsWith("/edit"))
      await page.locator("body").evaluate((element) => {
        element.scrollTop = 200;
      });
    await target.hover({ position: { x: 70, y: 100 } });
    const preview = page.getByRole("tooltip");
    await expect(preview).toBeVisible();
    const before = (await preview.boundingBox())!;
    const anchor = (await target.boundingBox())!;
    const opens_left = (await preview.getAttribute("data-side")) === "left";
    const gap = opens_left
      ? anchor.x - before.x - before.width
      : before.x - anchor.x - anchor.width;
    expect(gap).toBeGreaterThanOrEqual(4);
    expect(gap).toBeLessThanOrEqual(7);
    await page.locator("body").evaluate((element) => {
      element.scrollTop += 40;
    });
    await expect
      .poll(async () => (await preview.boundingBox())?.y)
      .toBeCloseTo(before.y - 40, 0);
    await page.mouse.move(1, 1);
    await expect(preview).toHaveCount(0);
  });

  for (const kind of ["shortage", "deck_usage"] as const) {
    test(`${kind} hint and arrow follow page scrolling at ${route}`, async ({
      page,
    }) => {
      const state = await mock_api(page);
      const current = state.deck.sections[1].cards[0];
      current.owned_quantity = kind === "shortage" ? 0 : 2;
      current.other_decks_quantity = kind === "shortage" ? 0 : 3;
      current.other_decks_count = kind === "shortage" ? 0 : 1;
      state.deck = with_summary(state.deck);
      await page.goto(route);
      const badge = page.getByRole("button", {
        name: kind === "shortage" ? "数量不足" : "卡组占用",
        exact: true,
      });
      await expect(badge).toBeVisible();
      // Keep the anchor away from viewport clamping while measuring movement.
      if (route.endsWith("/edit"))
        await page.locator("body").evaluate((element) => {
          element.scrollTop = 120;
        });
      await badge.focus();
      const hint = page.locator("[data-exception-side]");
      await expect(hint).toBeVisible();
      const before = (await hint.boundingBox())!;
      await page.locator("body").evaluate((element) => {
        element.scrollTop += 40;
      });
      await expect
        .poll(async () => (await hint.boundingBox())?.y)
        .toBeCloseTo(before.y - 40, 0);
      const tab = (await badge.boundingBox())!,
        arrow = (await hint.locator("svg").first().boundingBox())!;
      expect(
        Math.abs(arrow.y + arrow.height / 2 - tab.y - tab.height / 2),
      ).toBeLessThanOrEqual(1);
    });
  }

  test(`card description follows section horizontal scrolling at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1].cards.push(
      context_card(context(2, { type_group: "other" }), 1),
    );
    await page.goto(route);
    const target = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await target.focus();
    const preview = page.getByRole("tooltip");
    await expect(preview).toBeVisible();
    const before = (await preview.boundingBox())!;
    await target
      .locator("xpath=ancestor::section[1]")
      .locator('[class*="regionScroll"]')
      .evaluate((element) => {
        element.scrollLeft = 100;
      });
    await expect
      .poll(async () => (await preview.boundingBox())?.x)
      .toBeCloseTo(before.x - 100, 0);
  });

  test(`focused exception follows layout changes above its card at ${route}`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    state.deck.sections[1].cards.push(
      context_card(context(2, { owned_quantity: 0 }), 1),
    );
    state.deck = with_summary(state.deck);
    await page.goto(route);
    const first = page.getByRole("button", {
      name: "测试卡牌 1，2 张",
      exact: true,
    });
    await first.hover({ position: { x: 70, y: 10 } });
    await expect(first.locator("..")).toHaveCSS("height", "210px");
    const badge = page.getByRole("button", { name: "数量不足", exact: true });
    await badge.focus();
    const hint = page.locator("[data-exception-side]");
    await expect(hint).toBeVisible();
    const before = (await hint.boundingBox())!;
    await page.mouse.move(1, 1);
    await expect(first.locator("..")).toHaveCSS("height", "30px");
    await expect
      .poll(async () => (await hint.boundingBox())?.y)
      .toBeCloseTo(before.y - 180, 0);
  });
}

test("multi-digit deck quantities fit inside the corner marker", async ({
  page,
}) => {
  const state = await mock_api(page);
  state.deck.sections[1].cards[0].quantity = 1234;
  await page.goto("/decks/1");
  const marker = page
    .getByRole("button", { name: "测试卡牌 1，1234 张", exact: true })
    .locator('[class*="quantity"]');
  await expect(marker).toBeVisible();
  const bounds = await marker.evaluate((element) => {
    const range = document.createRange();
    range.selectNodeContents(element);
    const text = range.getBoundingClientRect();
    const box = element.getBoundingClientRect();
    return {
      left: text.left - box.left,
      top: text.top - box.top,
      diagonal: text.right - box.left + text.bottom - box.top,
      size: box.width,
    };
  });
  expect(bounds.left).toBeGreaterThanOrEqual(0);
  expect(bounds.top).toBeGreaterThanOrEqual(0);
  expect(bounds.diagonal).toBeLessThanOrEqual(bounds.size);
});

test("add-card preview follows its focused name while results scroll", async ({
  page,
}) => {
  await mock_api(page);
  await page.route("**/api/v1/decks/1/cards/search**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "preview-test",
        data: {
          items: Array.from({ length: 30 }, (_, index) => ({
            card: card(index + 1),
            faces: context(index + 1).faces,
          })),
          total: 30,
          next_cursor: null,
        },
      },
    }),
  );
  await page.goto("/decks/1/edit");
  await page.getByRole("button", { name: "增加卡牌", exact: true }).click();
  const modal = page.getByRole("dialog", { name: "增加卡牌", exact: true });
  await modal.getByRole("button", { name: "搜索", exact: true }).click();
  await modal.getByRole("button", { name: "测试卡牌 5", exact: true }).focus();
  const preview = page.getByRole("tooltip");
  await expect(preview).toBeVisible();
  const before = (await preview.boundingBox())!;
  await modal.locator('[class*="results"]').evaluate((element) => {
    element.scrollTop += 40;
  });
  await expect
    .poll(async () => (await preview.boundingBox())?.y)
    .toBeCloseTo(before.y - 40, 0);
});
