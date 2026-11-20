import { test, expect, type Page } from "@playwright/test";
import { mock_api } from "./mock-api";
import { card, detail, uuid } from "../src/test/fixtures";

async function scroll_body(page: Page, top: number) {
  const expected = await page.evaluate((value) => {
    document.body.scrollTo({ top: value, behavior: "instant" });
    return document.body.scrollTop;
  }, top);
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(expected);
  return expected;
}

async function mock_details(page: Page) {
  const initial = detail();
  initial.faces[0]!.display_oracle_text = "卡牌规则说明。\n".repeat(60);
  const language = {
    ...initial.languages[0]!,
    printing_uuid: uuid(2),
    language: { code: "ja", sort_order: 2 },
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
  for (const id of [uuid(1), uuid(2)])
    await page.route(`**/api/v1/cards/${id}`, (route) =>
      route.fulfill({
        json: {
          status: 200,
          error: null,
          request_id: "test",
          data: { ...initial, card: { ...initial.card, printing_uuid: id } },
        },
      }),
    );
}

test("cached detail navigation starts at top and history restores each body position", async ({
  page,
}) => {
  await mock_api(page);
  await mock_details(page);
  await page.goto("/sets/KLD/cards?language=en");
  const link = page.getByRole("link", { name: "查看测试卡牌 1", exact: true });
  await link.click();
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await expect(
    page.getByRole("heading", { name: "规则文字", exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(link).toBeVisible();
  const list_position = await scroll_body(page, 700);
  expect(list_position).toBe(700);
  await link.evaluate((element: HTMLAnchorElement) => element.click());
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await expect(
    page.getByRole("heading", { name: "规则文字", exact: true }),
  ).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  const detail_position = await scroll_body(page, 450);
  expect(detail_position).toBe(450);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(list_position);
  await page.goForward();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(detail_position);
});

test("history restoration waits for a refetched and locally expanded list", async ({
  page,
}) => {
  await mock_api(page);
  await mock_details(page);
  await page.goto("/sets/KLD/cards?language=en");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  await scroll_body(page, 100_000);
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 42", exact: true }),
  ).toHaveCount(1);
  const position = await scroll_body(page, 100_000);
  expect(position).toBeGreaterThan(2000);
  await page
    .getByRole("link", { name: "查看测试卡牌 1", exact: true })
    .evaluate((element: HTMLAnchorElement) => element.click());
  await expect(page).toHaveURL(`/cards/${uuid(1)}`);
  await page.evaluate(async () => {
    const path = "/src/api/query.ts";
    const { query_client } = await import(/* @vite-ignore */ path);
    query_client.removeQueries({ queryKey: ["cards", "set"] });
  });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/cards?**", async (route) => {
    await pending;
    await route.fallback();
  });
  await page.goBack();
  await expect(page.locator("[data-scroll-loading]")).toBeVisible();
  release();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 42", exact: true }),
  ).toHaveCount(1);
});

for (const action of ["language", "version"] as const) {
  test(`card ${action} updates preserve the body scroll position`, async ({
    page,
  }) => {
    await mock_api(page);
    await mock_details(page);
    await page.goto(`/cards/${uuid(1)}`);
    await expect(
      page.getByRole("combobox", { name: "当前系列语言" }),
    ).toBeVisible();
    const position = await scroll_body(page, 500);
    expect(position).toBe(500);
    if (action === "language")
      await page
        .getByRole("combobox", { name: "当前系列语言" })
        .evaluate((element: HTMLSelectElement, id) => {
          element.value = id;
          element.dispatchEvent(new Event("change", { bubbles: true }));
        }, uuid(2));
    else
      await page
        .getByRole("link", { name: "日文", exact: true })
        .evaluate((element: HTMLAnchorElement) => element.click());
    await expect(page).toHaveURL(`/cards/${uuid(2)}`);
    await expect
      .poll(() => page.evaluate(() => document.body.scrollTop))
      .toBe(position);
  });
}

test("global search resets the same search page while back restores its scroll", async ({
  page,
}) => {
  await mock_api(page);
  await page.route("**/api/v1/search?**", (route) =>
    route.fulfill({
      json: {
        status: 200,
        error: null,
        request_id: "test",
        data: {
          input: "test",
          mode: "name",
          executed_query: "test",
          items: Array.from({ length: 42 }, (_, index) => card(index + 1)),
          total: 42,
          next_cursor: null,
        },
      },
    }),
  );
  await page.goto("/search?q=first");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  const position = await scroll_body(page, 700);
  const search = page.getByRole("textbox", { name: "搜索卡牌或输入语法…" });
  await search.fill("second");
  await search.press("Enter");
  await expect(page).toHaveURL("/search?q=second");
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
});

test("list filters still reset to top and history restores the previous list position", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/sets/KLD/cards?language=en");
  await expect(
    page.getByRole("link", { name: "查看测试卡牌 1", exact: true }),
  ).toBeVisible();
  const position = await scroll_body(page, 700);
  await page
    .getByRole("combobox", { name: "排序", exact: true })
    .selectOption("name_desc");
  await expect(page).toHaveURL(/sort=name_desc/);
  await expect.poll(() => page.evaluate(() => document.body.scrollTop)).toBe(0);
  await page.goBack();
  await expect
    .poll(() => page.evaluate(() => document.body.scrollTop))
    .toBe(position);
});
