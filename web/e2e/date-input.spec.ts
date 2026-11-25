import { test, expect } from "@playwright/test";
import { mock_api } from "./mock-api";

test("end calendar starts at the start date and previews the shared interval without submitting", async ({
  page,
}, testInfo) => {
  await mock_api(page);
  let requests = 0;
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/v1/operation_logs/summary")
      requests++;
  });
  await page.goto("/operation_logs?start_date=2026-10-10");
  await expect(page.locator("main tbody tr")).toHaveCount(1);
  const initial = requests;
  const end = page.getByLabel("结束日期", { exact: true });
  const trigger = page.getByRole("button", {
    name: "选择结束日期",
    exact: true,
  });
  await trigger.click();
  const calendar = page.getByRole("dialog", {
    name: "结束日期日历",
    exact: true,
  });
  await expect(calendar).toContainText("2026年10月");
  const start_day = calendar.locator('[data-day="2026-10-10"]');
  const middle = calendar.locator('[data-day="2026-10-15"]');
  const end_day = calendar.locator('[data-day="2026-10-20"] button');
  await expect(start_day.locator("button")).toHaveCSS(
    "background-color",
    "rgb(35, 122, 80)",
  );
  await end_day.hover();
  await expect(middle).toHaveCSS("background-color", "rgb(227, 238, 231)");
  await expect(end_day).toHaveCSS("background-color", "rgb(35, 122, 80)");
  await expect(end).toHaveValue("");
  await page.getByRole("heading", { name: "操作日志", exact: true }).hover();
  await expect(middle).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await end_day.hover();
  await page.screenshot({ path: testInfo.outputPath("range-preview.png") });
  await end_day.click();
  await expect(end).toHaveValue("2026-10-20");
  await trigger.click();
  await expect(middle).toHaveCSS("background-color", "rgb(227, 238, 231)");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "选择开始日期", exact: true }).click();
  const start_calendar = page.getByRole("dialog", {
    name: "开始日期日历",
    exact: true,
  });
  await expect(start_calendar.locator('[data-day="2026-10-15"]')).toHaveCSS(
    "background-color",
    "rgb(227, 238, 231)",
  );
  await expect(end).toHaveValue("2026-10-20");
  expect(requests).toBe(initial);
  await expect(page).toHaveURL("/operation_logs?start_date=2026-10-10");
});

test("cross-year intervals preview keyboard changes and restore the saved end on dismissal", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/operation_logs?start_date=2026-12-30&end_date=2027-01-03");
  const end = page.getByLabel("结束日期", { exact: true });
  const trigger = page.getByRole("button", {
    name: "选择结束日期",
    exact: true,
  });
  await trigger.click();
  const calendar = page.getByRole("dialog", {
    name: "结束日期日历",
    exact: true,
  });
  await expect(calendar).toContainText("2027年1月");
  await expect(calendar.locator('[data-day="2027-01-01"]')).toHaveCSS(
    "background-color",
    "rgb(227, 238, 231)",
  );
  await expect(calendar.locator('[data-day="2026-12-30"]')).toHaveCSS(
    "opacity",
    "1",
  );
  await calendar.locator('[data-day="2027-01-05"] button').hover();
  await expect(calendar.locator('[data-day="2027-01-04"]')).toHaveCSS(
    "background-color",
    "rgb(227, 238, 231)",
  );
  await expect(calendar.locator('[data-day="2027-01-03"] button')).toHaveCSS(
    "background-color",
    "rgba(0, 0, 0, 0)",
  );
  await expect(end).toHaveValue("2027-01-03");
  await page.keyboard.press("Escape");
  await trigger.click();
  await expect(calendar.locator('[data-day="2027-01-04"]')).toHaveCSS(
    "background-color",
    "rgba(0, 0, 0, 0)",
  );
  await page.keyboard.press("ArrowRight");
  await expect(calendar.locator('[data-day="2027-01-03"]')).toHaveCSS(
    "background-color",
    "rgb(227, 238, 231)",
  );
  await expect(end).toHaveValue("2027-01-03");
  await page.keyboard.press("Enter");
  await expect(end).toHaveValue("2027-01-04");
});

test("same-day intervals can be selected and invalid starts do not draw a range", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/operation_logs?start_date=2026-10-10");
  const start = page.getByLabel("开始日期", { exact: true });
  const end = page.getByLabel("结束日期", { exact: true });
  const trigger = page.getByRole("button", {
    name: "选择结束日期",
    exact: true,
  });
  await trigger.click();
  const calendar = page.getByRole("dialog", {
    name: "结束日期日历",
    exact: true,
  });
  await calendar.locator('[data-day="2026-10-10"] button').click();
  await expect(end).toHaveValue("2026-10-10");
  await trigger.click();
  await expect(calendar.locator('[data-day="2026-10-10"]')).toHaveCSS(
    "background-image",
    "none",
  );
  await page.keyboard.press("Escape");
  for (const value of ["2026-10-20", "2026-", ""]) {
    await start.fill(value);
    await trigger.click();
    await expect(calendar.locator('[class*="rangeMiddle"]')).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(end).toHaveValue("2026-10-10");
  }
});

test("Chinese calendars update each field without submitting the date query", async ({
  page,
}, testInfo) => {
  await mock_api(page);
  const requests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === "/api/v1/operation_logs/summary")
      requests.push(request.url());
  });
  await page.goto("/operation_logs?start_date=2026-10-01&end_date=2026-10-31");
  await expect(page.locator("main tbody tr")).toHaveCount(1);
  const initial = requests.length;
  await page.getByRole("button", { name: "选择开始日期", exact: true }).click();
  const calendar = page.getByRole("dialog", {
    name: "开始日期日历",
    exact: true,
  });
  await expect(calendar).toBeVisible();
  await expect(calendar).toContainText("2026年10月");
  await expect(calendar.locator("thead th")).toHaveCount(7);
  await expect(calendar.locator("thead th").first()).toHaveCSS(
    "background-color",
    "rgba(0, 0, 0, 0)",
  );
  await expect(calendar).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await expect(calendar).toHaveCSS("border-radius", "8px");
  await page.screenshot({ path: testInfo.outputPath("calendar.png") });
  await calendar.locator("button").filter({ hasText: /^15$/ }).click();
  await expect(calendar).toHaveCount(0);
  await expect(page.getByLabel("开始日期", { exact: true })).toHaveValue(
    "2026-10-15",
  );
  await page.getByRole("button", { name: "选择结束日期", exact: true }).click();
  const end_calendar = page.getByRole("dialog", {
    name: "结束日期日历",
    exact: true,
  });
  await end_calendar.locator("button").filter({ hasText: /^20$/ }).click();
  await expect(page.getByLabel("结束日期", { exact: true })).toHaveValue(
    "2026-10-20",
  );
  expect(requests).toHaveLength(initial);
  await expect(page).toHaveURL(/start_date=2026-10-01&end_date=2026-10-31$/);
  await page.getByRole("button", { name: "查询", exact: true }).click();
  await expect(page).toHaveURL(/start_date=2026-10-15.*end_date=2026-10-20/);
});

test("calendar keyboard selection, dismissal and year navigation preserve the date", async ({
  page,
}) => {
  await mock_api(page);
  await page.goto("/operation_logs?start_date=2026-12-31");
  const input = page.getByLabel("开始日期", { exact: true });
  const trigger = page.getByRole("button", {
    name: "选择开始日期",
    exact: true,
  });
  await trigger.click();
  const calendar = page.getByRole("dialog", {
    name: "开始日期日历",
    exact: true,
  });
  await expect(calendar.locator("button:focus")).toHaveText("31");
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("Enter");
  await expect(input).toHaveValue("2027-01-01");
  await expect(calendar).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await trigger.click();
  await calendar.getByRole("button", { name: "上个月", exact: true }).click();
  await expect(calendar).toContainText("2026年12月");
  await page.keyboard.press("Escape");
  await expect(calendar).toHaveCount(0);
  await expect(input).toHaveValue("2027-01-01");
  await trigger.click();
  await expect(calendar).toContainText("2027年1月");
  await page.getByRole("heading", { name: "操作日志", exact: true }).click();
  await expect(calendar).toHaveCount(0);
  await expect(input).toHaveValue("2027-01-01");
});

test("calendar stays within a short desktop viewport and allows scrolling", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 420 });
  await mock_api(page);
  await page.goto("/operation_logs?start_date=2026-10-01");
  await page.getByRole("button", { name: "选择开始日期", exact: true }).click();
  const calendar = page.getByRole("dialog", {
    name: "开始日期日历",
    exact: true,
  });
  const box = (await calendar.boundingBox())!;
  expect(box.y).toBeGreaterThanOrEqual(8);
  expect(box.y + box.height).toBeLessThanOrEqual(412);
  const day = calendar.locator("button").filter({ hasText: /^31$/ });
  await day.scrollIntoViewIfNeeded();
  await day.click();
  await expect(page.getByLabel("开始日期", { exact: true })).toHaveValue(
    "2026-10-31",
  );
});
