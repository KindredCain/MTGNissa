import { describe, expect, it } from "vitest";
import { card } from "../test/fixtures";
import { compare_collectors, filter_cards } from "./model";
describe("full Set result filtering", () => {
  it("sorts long numeric collector identifiers without losing precision", () => {
    const values = [
      "10b",
      "2",
      "10a",
      "10",
      "special",
      "9999999999999999999999998",
      "9999999999999999999999999",
    ];
    expect(values.sort(compare_collectors)).toEqual([
      "2",
      "10",
      "10a",
      "10b",
      "9999999999999999999999998",
      "9999999999999999999999999",
      "special",
    ]);
  });
  it("combines filters across the full result and supports reverse sorting", () => {
    const cards = [
      card(1, { colors: ["W", "U"], cmc: 2 }),
      card(2, { colors: [], cmc: 2 }),
      card(3, { colors: ["W", "U"], cmc: 2 }),
    ];
    const result = filter_cards(
      cards,
      new URLSearchParams(
        "color=multicolor&mana_value=2&sort=collector_number_desc",
      ),
    );
    expect(result.map((card) => card.collector_number)).toEqual(["3", "1"]);
    expect(
      filter_cards(cards, new URLSearchParams("color=colorless"))[0]
        .collector_number,
    ).toBe("2");
  });
});
