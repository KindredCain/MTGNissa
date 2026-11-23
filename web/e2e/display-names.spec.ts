import { expect, test } from "@playwright/test";
import { card, detail, options, uuid } from "../src/test/fixtures";
import { mock_api } from "./mock-api";

test("sort names remain scoped to their pages and match the current parameter values", async ({
  page,
}) => {
  await mock_api(page);
  const scenes = [
    {
      route: "/sets",
      control: "系列排序",
      initial: "released_desc",
      initial_name: "发行日期 ↓",
      target: "code_desc",
      target_name: "简写 ↓",
    },
    {
      route: "/sets/KLD/cards?language=en",
      control: "排序",
      initial: "collector_number_asc",
      initial_name: "收藏编号 ↑",
      target: "name_desc",
      target_name: "名称 ↓",
    },
    {
      route: "/collection",
      control: "排序",
      initial: "set_desc",
      initial_name: "系列发行 ↓",
      target: "mana_value_asc",
      target_name: "法术力值 ↑",
    },
    {
      route: "/wishlist",
      control: "排序",
      initial: "set_desc",
      initial_name: "系列发行 ↓",
      target: "name_asc",
      target_name: "名称 ↑",
    },
    {
      route: "/decks",
      control: "卡组排序",
      initial: "updated_desc",
      initial_name: "更新时间 ↓",
      target: "created_desc",
      target_name: "创建时间 ↓",
    },
  ];
  for (const scene of scenes) {
    await page.goto(scene.route);
    const select = page.getByRole("combobox", {
      name: scene.control,
      exact: true,
    });
    await expect(select).toHaveValue(scene.initial);
    await expect(select.locator("option:checked")).toContainText(
      scene.initial_name,
    );
    await select.selectOption({ label: `排序：${scene.target_name}` });
    await expect(select).toHaveValue(scene.target);
    await expect
      .poll(() => new URL(page.url()).searchParams.get("sort"))
      .toBe(scene.target);
  }
});

test("language filters use API codes and preserve the returned options", async ({
  page,
}) => {
  await mock_api(page);
  for (const kind of ["collection", "wishlist"])
    await page.route(`**/api/v1/${kind}/options`, (route) =>
      route.fulfill({
        json: {
          status: 200,
          data: {
            ...options,
            languages: [
              { code: "en", sort_order: 1 },
              { code: "ja", sort_order: 2 },
            ],
          },
          error: null,
          request_id: "test",
        },
      }),
    );
  for (const route of ["/sets", "/collection", "/wishlist"]) {
    await page.goto(route);
    await page.getByRole("button", { name: "语言", exact: true }).click();
    await expect(
      page.getByRole("checkbox", { name: "英文", exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("checkbox")).toHaveCount(3);
    await page.getByRole("checkbox", { name: "英文", exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/language=en/);
  }
});

test("special language names render from codes in breadcrumbs and version options", async ({
  page,
}) => {
  await mock_api(page);
  const current = card(1, { language: { code: "ph", sort_order: 17 } });
  await page.route("**/api/v1/cards?**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        data: {
          set: current.set,
          language: current.language,
          items: [current],
          total: 1,
        },
        error: null,
        request_id: "test",
      },
    }),
  );
  const data = detail();
  data.card.language = current.language;
  data.languages[0]!.language = current.language;
  data.other_versions = [
    {
      set: { ...current.set, code: "TST" },
      collector_number: "2",
      released_at: current.released_at,
      normal_quantity: 0,
      foil_quantity: 0,
      languages: [{ ...data.languages[0]!, printing_uuid: uuid(2) }],
    },
  ];
  await page.route(`**/api/v1/cards/${uuid(1)}`, (route) =>
    route.fulfill({
      json: { status: 200, data, error: null, request_id: "test" },
    }),
  );

  await page.goto("/sets/KLD/cards?language=ph");
  await expect(
    page.getByText("卡拉德许（非瑞）", { exact: true }),
  ).toBeVisible();
  await page.goto(`/cards/${uuid(1)}`);
  await expect(
    page.getByRole("button", { name: "增加普通", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "增加闪卡", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "卡拉德许（非瑞）", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("combobox", { name: "当前系列语言" }),
  ).toContainText("非瑞");
  await expect(
    page.getByRole("link", { name: "非瑞", exact: true }),
  ).toBeVisible();
});

for (const scene of [
  {
    route: "/sets",
    endpoint: "/api/v1/sets",
    prefix: "sets",
    filters: [
      ["语言", "language"],
      ["系列类型", "set_type"],
    ],
  },
  {
    route: "/sets/KLD/cards?language=en",
    endpoint: "/api/v1/cards",
    prefix: null,
    filters: [
      ["颜色", "color"],
      ["法术力", "mana_value"],
      ["稀有度", "rarity"],
    ],
  },
  {
    route: "/collection",
    endpoint: "/api/v1/collection",
    prefix: "collection",
    filters: [
      ["系列", "set_code"],
      ["语言", "language"],
      ["颜色", "color"],
      ["法术力", "mana_value"],
      ["稀有度", "rarity"],
    ],
  },
  {
    route: "/wishlist",
    endpoint: "/api/v1/wishlist",
    prefix: "wishlist",
    filters: [
      ["系列", "set_code"],
      ["语言", "language"],
      ["颜色", "color"],
      ["法术力", "mana_value"],
      ["稀有度", "rarity"],
    ],
  },
] as const) {
  test(`all multi-select filters omit complete selections at ${scene.route}`, async ({
    page,
  }) => {
    await mock_api(page);
    const requests: URL[] = [];
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.pathname === scene.endpoint) requests.push(url);
    });
    await page.goto(scene.route);
    await expect(page.locator("[data-scroll-loading]")).toHaveCount(0);
    for (const [label, parameter] of scene.filters) {
      const trigger = page.getByRole("button", { name: label, exact: true });
      await trigger.click();
      const choices = page.getByRole("checkbox");
      await expect.poll(() => choices.count()).toBeGreaterThan(1);
      const count = await choices.count();
      if (count > 2) {
        const first_name = (await choices.nth(1).getAttribute("aria-label"))!;
        await choices.nth(1).click();
        await page.keyboard.press("Escape");
        await expect
          .poll(() => new URL(page.url()).searchParams.has(parameter))
          .toBe(true);
        await expect(trigger).toContainText(first_name);
        await trigger.click();
        if (scene.prefix)
          await page.evaluate(async (prefix) => {
            const path = "/src/api/query.ts";
            const { query_client } = await import(/* @vite-ignore */ path);
            await query_client.invalidateQueries({
              queryKey: [prefix, "list"],
              refetchType: "none",
            });
          }, scene.prefix);
      }
      for (let index = 1; index < count; index++)
        if ((await choices.nth(index).getAttribute("aria-checked")) !== "true")
          await choices.nth(index).click();
      const before = requests.length;
      await page.keyboard.press("Escape");
      await expect(choices).toHaveCount(0);
      await expect(trigger).toContainText("全部");
      expect(new URL(page.url()).searchParams.has(parameter)).toBe(false);
      if (!scene.prefix || count === 2) expect(requests).toHaveLength(before);
      else {
        expect(requests.length).toBeGreaterThan(before);
        expect(requests.at(-1)!.searchParams.has(parameter)).toBe(false);
      }
    }
  });
}

test("selecting every mana value retains fractional cards without refetching", async ({
  page,
}) => {
  await mock_api(page);
  let requests = 0;
  await page.route("**/api/v1/cards?**", (route) => {
    requests++;
    return route.fulfill({
      json: {
        status: 200,
        data: {
          set: card().set,
          language: card().language,
          items: [card(1, { cmc: 0.5 }), card(2, { cmc: 1 })],
          total: 2,
        },
        error: null,
        request_id: "test",
      },
    });
  });
  await page.goto("/sets/KLD/cards?language=en");
  const fractional = page.getByRole("link", {
    name: "查看测试卡牌 1",
    exact: true,
  });
  await expect(fractional).toBeVisible();
  const initial_requests = requests;
  const trigger = page.getByRole("button", { name: "法术力", exact: true });
  for (const from_partial of [false, true]) {
    if (from_partial) {
      await trigger.click();
      await page.getByRole("checkbox", { name: "1", exact: true }).click();
      await page.keyboard.press("Escape");
      await expect(page).toHaveURL(/mana_value=1/);
      await expect(fractional).toHaveCount(0);
    }
    await trigger.click();
    for (const label of ["0", "1", "2", "3", "4", "5", "6", "7", "8+"]) {
      const choice = page.getByRole("checkbox", { name: label, exact: true });
      if ((await choice.getAttribute("aria-checked")) !== "true")
        await choice.click();
    }
    await page.keyboard.press("Escape");
    await expect(fractional).toBeVisible();
    await expect(trigger).toContainText("全部");
    expect(new URL(page.url()).searchParams.has("mana_value")).toBe(false);
    expect(requests).toBe(initial_requests);
  }
});
