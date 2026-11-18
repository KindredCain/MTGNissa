import { expect, test } from "@playwright/test";
import { card, detail, uuid } from "../src/test/fixtures";
import { mock_api } from "./mock-api";

test("sort names remain scoped to their pages and keep the original parameter values", async ({
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
      initial: "set",
      initial_name: "系列发行 ↓",
      target: "mana_value",
      target_name: "法术力值 ↑",
    },
    {
      route: "/wishlist",
      control: "排序",
      initial: "set",
      initial_name: "系列发行 ↓",
      target: "name",
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
  for (const route of ["/sets", "/collection", "/wishlist"]) {
    await page.goto(route);
    await page.getByRole("button", { name: "语言", exact: true }).click();
    await expect(
      page.getByRole("checkbox", { name: "英文", exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("checkbox")).toHaveCount(
      route === "/sets" ? 3 : 2,
    );
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
