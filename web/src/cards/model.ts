import type { DTO } from "../api/client";
import type { Option } from "../ui/controls";
export type Card = DTO<"CardSummaryResponse">;
export const color_names = {
  W: "白色",
  U: "蓝色",
  B: "黑色",
  R: "红色",
  G: "绿色",
  C: "无色",
} as const;
export const color_options: Option[] = [
  { value: "W", label: color_names.W },
  { value: "U", label: color_names.U },
  { value: "B", label: color_names.B },
  { value: "R", label: color_names.R },
  { value: "G", label: color_names.G },
  { value: "multicolor", label: "多色" },
  { value: "colorless", label: color_names.C },
];
export const mana_options: Option[] = Array.from({ length: 9 }, (_, index) => ({
  value: index === 8 ? "8_plus" : String(index),
  label: index === 8 ? "8+" : String(index),
}));
export const rarity_options: Option[] = [
  "common",
  "uncommon",
  "rare",
  "mythic",
  "special",
  "bonus",
].map((value, index) => ({
  value,
  label: ["铁", "银", "金", "秘稀", "特殊", "额外"][index],
}));
export const list_sort: Option[] = [
  { value: "set", label: "系列" },
  { value: "color", label: "颜色" },
  { value: "mana_value", label: "法术力" },
  { value: "name", label: "名称" },
];
export const local_sort: Option[] = [
  { value: "collector_number", label: "收藏编号" },
  { value: "collector_number_desc", label: "收藏编号：降序" },
  { value: "mana_value", label: "法术力：从低到高" },
  { value: "mana_value_desc", label: "法术力：从高到低" },
  { value: "name", label: "名称：A–Z" },
  { value: "name_desc", label: "名称：Z–A" },
];
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
    const selected = params.get("sort") ?? "collector_number",
      descending = selected.endsWith("_desc"),
      sort = selected.replace(/_desc$/, "");
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
