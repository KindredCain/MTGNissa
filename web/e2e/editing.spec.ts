import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { context, uuid } from "../src/test/fixtures";

for (const outcome of ["success", "failure"] as const) {
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
    await old_modal.getByRole("button", { name: "×", exact: true }).click();
    await page.getByRole("link", { name: "卡组", exact: true }).click();
    await expect(page).toHaveURL("/decks");
    await page.getByRole("link", { name: "查看测试卡组", exact: true }).click();
    await page.getByRole("link", { name: "编辑卡组", exact: true }).click();
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
  await mock_api(page);
  for (const held of [false, true]) {
    await page.goto("/decks/1/edit");
    await page.getByRole("button", { name: "卡组代码", exact: false }).click();
    const source = page.getByRole("button", { name: "测试卡牌 1，2 张" });
    await expect(source).toBeVisible();
    const region = page.locator("section").filter({
      has: page.getByRole("heading", { name: "备牌", exact: true }),
    });
    const source_box = (await source.boundingBox())!,
      target = (await region.boundingBox())!;
    await page.mouse.move(source_box.x + 75, source_box.y + 90);
    await page.mouse.down();
    if (held) await page.waitForTimeout(400);
    await page.mouse.move(source_box.x + 81, source_box.y + 90);
    await page.mouse.move(target.x + 420, target.y + 100, { steps: 8 });
    await page.mouse.up();
    await expect(
      region.getByRole("button", { name: `测试卡牌 1，${held ? 2 : 1} 张` }),
    ).toBeVisible();
    expect(
      await page
        .getByRole("textbox", { name: "卡组代码", exact: true })
        .count(),
    ).toBe(0);
  }
});
