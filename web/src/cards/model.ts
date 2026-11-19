import type { DTO, QueryParams } from "../api/client";
import type { Option } from "../ui/controls";
import {
  card_sort_names,
  color_names,
  rarity_names,
} from "../shared/display-names";
export type Card = DTO<"CardSummaryResponse">;
export const color_options: Option[] = [
  { value: "W", label: color_names.W },
  { value: "U", label: color_names.U },
  { value: "B", label: color_names.B },
  { value: "R", label: color_names.R },
  { value: "G", label: color_names.G },
  { value: "multicolor", label: color_names.multicolor },
  { value: "colorless", label: color_names.colorless },
];
export const mana_options: Option[] = Array.from({ length: 9 }, (_, index) => ({
  value: index === 8 ? "8_plus" : String(index),
  label: index === 8 ? "8+" : String(index),
}));
export const rarity_options: Option[] = (
  ["common", "uncommon", "rare", "mythic", "special", "bonus"] as const
).map((value) => ({
  value,
  label: rarity_names[value],
}));
export const collection_sorts = [
  "set_desc",
  "color",
  "mana_value_asc",
  "name_asc",
] as const satisfies readonly Exclude<
  NonNullable<QueryParams<"/api/v1/collection">>["sort"],
  undefined | ""
>[];
export const wishlist_sorts = [
  "set_desc",
  "color",
  "mana_value_asc",
  "name_asc",
] as const satisfies readonly Exclude<
  NonNullable<QueryParams<"/api/v1/wishlist">>["sort"],
  undefined | ""
>[];
export const local_sort: Option[] = (
  [
    "collector_number_desc",
    "collector_number_asc",
    "mana_value_desc",
    "mana_value_asc",
    "name_desc",
    "name_asc",
  ] as const
).map((value) => ({ value, label: card_sort_names[value] }));
const natural = new Intl.Collator("en", { numeric: true, sensitivity: "base" });
export function compare_collectors(a: string, b: string) {
  const left = a.match(/^(\d+)(.*)$/),
    right = b.match(/^(\d+)(.*)$/);
  if (!left || !right) return left ? -1 : right ? 1 : natural.compare(a, b);
  const a_number = BigInt(left[1]),
    b_number = BigInt(right[1]);
  return (
    (a_number < b_number ? -1 : a_number > b_number ? 1 : 0) ||
    natural.compare(left[2], right[2])
  );
}
export function filter_cards(cards: Card[], params: URLSearchParams) {
  const colors = params.getAll("color"),
    mana = params.getAll("mana_value"),
    rarities = params.getAll("rarity");
  const filtered = cards.filter(
    (card) =>
      (!colors.length ||
        colors.some((color) =>
          color === "colorless"
            ? !card.colors.length
            : color === "multicolor"
              ? card.colors.length >= 2
              : card.colors.length === 1 &&
                card.colors.some((value) => value === color),
        )) &&
      (!mana.length ||
        mana.some((value) =>
          value === "8_plus" ? card.cmc >= 8 : card.cmc === Number(value),
        )) &&
      (!rarities.length || rarities.includes(card.rarity)),
  );
  return filtered.sort((a, b) => {
    const selected = params.get("sort") ?? "collector_number_asc",
      descending = selected.endsWith("_desc"),
      sort = selected.replace(/_(asc|desc)$/, "");
    if (
      sort === "collector_number" &&
      /^\d/.test(a.collector_number) !== /^\d/.test(b.collector_number)
    )
      return /^\d/.test(a.collector_number) ? -1 : 1;
    const primary =
      sort === "name"
        ? natural.compare(a.name, b.name)
        : sort === "mana_value"
          ? a.cmc - b.cmc
          : compare_collectors(a.collector_number, b.collector_number);
    return (
      primary * (descending ? -1 : 1) ||
      compare_collectors(a.collector_number, b.collector_number) ||
      a.printing_uuid.localeCompare(b.printing_uuid)
    );
  });
}
