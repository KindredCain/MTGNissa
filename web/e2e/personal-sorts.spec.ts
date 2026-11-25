import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";
import { personal_sort_names } from "../src/shared/display-names";

for (const kind of ["collection", "wishlist"] as const) {
  test(`${kind} sort values follow displayed arrows and retain set grouping behavior`, async ({
    page,
  }) => {
    await mock_api(page);
    const requests: URL[] = [];
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.pathname === `/api/v1/${kind}`) requests.push(url);
    });
    await page.goto(`/${kind}`);
    const sort = page.getByRole("combobox", { name: "排序", exact: true });
    const group = page.getByRole("button", { name: /KLD.*卡拉德许/ });
    await expect(sort).toContainText("系列发行 ↓");
    await expect(group).toBeVisible();
    expect(
      requests.some((url) => url.searchParams.get("sort") === "set_desc"),
    ).toBe(true);
    await sort.click();
    await expect(page.getByRole("option")).toHaveText([
      "系列发行 ↓",
      "颜色",
      "法术力值 ↑",
      "名称 ↑",
    ]);
    await page.keyboard.press("Escape");
    if (kind === "collection")
      await expect(
        page.getByRole("switch", { name: "合并不同系列" }),
      ).toBeDisabled();
    for (const value of ["color", "mana_value_asc", "name_asc"] as const) {
      await sort.click();
      await page
        .getByRole("option", {
          name: personal_sort_names[kind][value],
          exact: true,
        })
        .click();
      await expect(sort).toContainText(personal_sort_names[kind][value]);
      await expect(page).toHaveURL(new RegExp(`sort=${value}`));
      await expect(group).toHaveCount(0);
      expect(
        requests.some((url) => url.searchParams.get("sort") === value),
      ).toBe(true);
      if (kind === "collection" && value === "color") {
        const merge = page.getByRole("switch", { name: "合并不同系列" });
        await expect(merge).toBeEnabled();
        await merge.click();
        await expect(merge).toBeChecked();
      }
    }
    await sort.click();
    await page.getByRole("option", { name: "系列发行 ↓", exact: true }).click();
    await expect(page).toHaveURL(/sort=set_desc/);
    await expect(group).toBeVisible();
    if (kind === "collection") {
      const merge = page.getByRole("switch", { name: "合并不同系列" });
      await expect(merge).toBeDisabled();
      await expect(merge).not.toBeChecked();
    } else {
      expect(new URL(page.url()).searchParams.has("merge_sets")).toBe(false);
      for (const request of requests) {
        expect(request.searchParams.has("merge_sets")).toBe(false);
        expect(request.searchParams.has("merge_languages")).toBe(false);
      }
    }
    await page.goBack();
    await expect(sort).toContainText("名称 ↑");
    await expect(group).toHaveCount(0);
    await page.goForward();
    await expect(sort).toContainText("系列发行 ↓");
    await expect(group).toBeVisible();
  });
}
