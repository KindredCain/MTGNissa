import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";

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
    await expect(sort).toHaveValue("set_desc");
    await expect(group).toBeVisible();
    expect(
      requests.some((url) => url.searchParams.get("sort") === "set_desc"),
    ).toBe(true);
    expect(
      await sort
        .locator("option")
        .evaluateAll((options) =>
          options.map((option) => (option as HTMLOptionElement).value),
        ),
    ).toEqual(["set_desc", "color", "mana_value_asc", "name_asc"]);
    if (kind === "collection")
      await expect(
        page.getByRole("switch", { name: "合并不同系列" }),
      ).toBeDisabled();
    for (const value of ["color", "mana_value_asc", "name_asc"]) {
      await sort.selectOption(value);
      await expect(sort).toHaveValue(value);
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
    await sort.selectOption("set_desc");
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
    await expect(sort).toHaveValue("name_asc");
    await expect(group).toHaveCount(0);
    await page.goForward();
    await expect(sort).toHaveValue("set_desc");
    await expect(group).toBeVisible();
  });
}
