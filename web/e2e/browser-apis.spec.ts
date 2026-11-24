import { expect, test } from "@playwright/test";
import { mock_api } from "./mock-api";
import { uuid } from "../src/test/fixtures";

test("editor and all idempotent actions work without native randomUUID", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() => {
    Object.defineProperty(crypto, "randomUUID", { value: undefined });
  });
  const state = await mock_api(page);
  const keys: string[] = [];
  page.on("request", (request) => {
    const key = request.headers()["idempotency-key"];
    if (key) keys.push(key);
  });
  await page.goto("/decks");
  await page.getByRole("button", { name: "新建卡组" }).click();
  await expect(page.getByRole("textbox", { name: "卡组名称" })).toBeVisible();
  await page.goto("/decks/1");
  await page.getByRole("button", { name: "复制卡组", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "卡组名称" })).toBeVisible();
  await page.goto(`/cards/${uuid(1)}`);
  await page.getByRole("button", { name: "增加普通", exact: true }).click();
  await expect.poll(() => state.normal).toBe(3);
  await page.getByRole("button", { name: "愿望单", exact: true }).click();
  await expect.poll(() => state.included).toBe(false);
  expect(keys).toHaveLength(4);
  expect(new Set(keys).size).toBe(4);
  for (const key of keys)
    expect(key).toMatch(
      /^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/,
    );
  expect(errors).toEqual([]);
});

for (const origin of ["/decks", "/decks/1"]) {
  test(`copying deck code from ${origin} works without the Clipboard API`, async ({
    page,
  }) => {
    const state = await mock_api(page);
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "clipboard", { value: undefined });
      document.addEventListener("copy", () => {
        document.body.dataset.copiedText = (
          document.activeElement as HTMLTextAreaElement
        ).value;
      });
    });
    await page.goto(origin);
    if (origin === "/decks")
      await page.getByRole("button", { name: "更多卡组操作" }).click();
    await page
      .getByRole("button", { name: "复制卡组代码", exact: true })
      .click();
    await expect(
      page.getByRole("status").filter({ hasText: "卡组代码已复制" }),
    ).toBeVisible();
    await expect(page.locator("body")).toHaveAttribute(
      "data-copied-text",
      state.deck.source_text,
    );
    await expect(page.locator("textarea")).toHaveCount(0);
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
}
