import { describe, expect, it } from "vitest";
import { format_date, parse_date } from "./date";

describe("calendar dates", () => {
  it.each([
    "2026-10-01",
    "2024-02-29",
    "2000-02-29",
    "0001-01-01",
    "9999-12-31",
  ])("preserves the calendar date %s", (value) => {
    const date = parse_date(value);
    expect(date).toBeInstanceOf(Date);
    expect(format_date(date!)).toBe(value);
  });
  it.each([
    "",
    "2026-10-",
    "2026-2-01",
    "2026-02-29",
    "1900-02-29",
    "2026-02-30",
    "2026-04-31",
    "2026-13-01",
    "2026-00-01",
    "2026-10-00",
    "0000-01-01",
    "2026-10-01T00:00:00Z",
    "2026-10-01 ",
    "not-a-date",
  ])("rejects invalid or incomplete dates: %s", (value) => {
    expect(parse_date(value)).toBeUndefined();
  });
});
