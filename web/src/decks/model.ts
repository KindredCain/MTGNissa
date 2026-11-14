import type { DTO } from "../api/client";
export type Deck = DTO<"DeckDetailResponse">;
export type DeckCard = DTO<"DeckCardResponse">;
export type SectionType = DTO<"SectionType">;
export const section_types: SectionType[] = [
  "commander",
  "mainboard",
  "sideboard",
  "maybeboard",
];
export const section_names: Record<SectionType, string> = {
  commander: "指挥官",
  mainboard: "主牌",
  sideboard: "备牌",
  maybeboard: "备选",
};
export const type_names: Record<DTO<"TypeGroup">, string> = {
  artifact: "神器",
  creature: "生物",
  enchantment: "结界",
  instant: "瞬间",
  sorcery: "法术",
  planeswalker: "鹏洛客",
  land: "地",
  other: "其他",
};
export const type_order = Object.keys(type_names) as DTO<"TypeGroup">[];
export const colors = ["W", "U", "B", "R", "G", "C"] as const;
export const palette = [
  "#FBF6D3",
  "#96C4D4",
  "#B0A8A3",
  "#D88F73",
  "#93CBA4",
  "#A9A8A8",
];
export function counted_cards(sections: Deck["sections"]) {
  return sections
    .filter(
      (section) => section.type === "commander" || section.type === "mainboard",
    )
    .flatMap((section) => section.cards);
}
export function with_summary(deck: Deck): Deck {
  const totals = new Map<string, number>();
  for (const section of deck.sections)
    if (section.type !== "maybeboard")
      for (const card of section.cards)
        totals.set(
          card.oracle_id,
          (totals.get(card.oracle_id) ?? 0) + card.quantity,
        );
  let counted_total = 0,
    collected = 0;
  const shortage = new Map<string, number>(),
    usage = new Map<string, number>();
  const unique = new Map(
    deck.sections
      .flatMap((section) => section.cards)
      .map((card) => [card.oracle_id, card]),
  );
  for (const [oracle, quantity] of totals) {
    const card = unique.get(oracle)!;
    counted_total += quantity;
    collected += Math.min(quantity, card.owned_quantity);
    if (quantity > card.owned_quantity)
      shortage.set(oracle, quantity - card.owned_quantity);
    else if (
      card.other_decks_count > 0 &&
      quantity + card.other_decks_quantity > card.owned_quantity
    )
      usage.set(
        oracle,
        quantity + card.other_decks_quantity - card.owned_quantity,
      );
  }
  const sections = deck.sections.map((section) => ({
    ...section,
    quantity: section.cards.reduce((sum, card) => sum + card.quantity, 0),
    cards: section.cards.map((card) => {
      const all_decks_quantity =
        (totals.get(card.oracle_id) ?? 0) + card.other_decks_quantity;
      const deficit = Math.max(
          0,
          (totals.get(card.oracle_id) ?? 0) - card.owned_quantity,
        ),
        occupied = Math.max(0, all_decks_quantity - card.owned_quantity);
      let exception: DeckCard["exception"] = null;
      if (section.type !== "maybeboard" && card.quantity > 0) {
        if (deficit > 0)
          exception = { type: "shortage", missing_quantity: deficit };
        else if (occupied > 0 && card.other_decks_count > 0)
          exception = { type: "deck_usage", missing_quantity: occupied };
      }
      return {
        ...card,
        all_decks_quantity,
        using_decks: card.other_decks_count + 1,
        exception,
      };
    }),
  }));
  return {
    ...deck,
    sections,
    summary: {
      counted_total,
      collection_rate: counted_total ? collected / counted_total : 0,
      shortage: {
        distinct_cards: shortage.size,
        missing_quantity: [...shortage.values()].reduce((a, b) => a + b, 0),
      },
      deck_usage: {
        distinct_cards: usage.size,
        missing_quantity: [...usage.values()].reduce((a, b) => a + b, 0),
      },
    },
  };
}
export function context_card(
  context: DTO<"DeckContextResponse">,
  quantity: number,
): DeckCard {
  return {
    ...context,
    quantity,
    all_decks_quantity: context.other_decks_quantity + quantity,
    using_decks: context.other_decks_count + 1,
    exception: null,
  };
}
export function deck_save(deck: Deck): DTO<"DeckSaveRequest"> {
  return {
    name: deck.name,
    cover_printing_uuid: deck.cover_printing_uuid,
    sections: deck.sections.map((section) => ({
      type: section.type,
      cards: section.cards
        .filter((card) => card.quantity > 0)
        .map((card) => ({
          printing_uuid: card.printing_uuid,
          quantity: card.quantity,
        })),
    })),
  };
}
const color_order = [
  "W",
  "U",
  "B",
  "R",
  "G",
  "WU",
  "UB",
  "BR",
  "RG",
  "GW",
  "WB",
  "UR",
  "BG",
  "RW",
  "GU",
  "WUB",
  "UBR",
  "BRG",
  "RGW",
  "GWU",
  "WBG",
  "URW",
  "BGU",
  "RWB",
  "GUR",
  "WUBR",
  "UBRG",
  "BRGW",
  "RGWU",
  "GWUB",
  "WUBRG",
  "",
];
function color_rank(card: DeckCard) {
  return color_order.findIndex(
    (combination) =>
      combination.length === card.card.colors.length &&
      card.card.colors.every((color) => combination.includes(color)),
  );
}
export function compare_cards(a: DeckCard, b: DeckCard) {
  return (
    a.statistics.cmc - b.statistics.cmc ||
    color_rank(a) - color_rank(b) ||
    a.card.name.localeCompare(b.card.name, "en", { sensitivity: "base" }) ||
    a.printing_uuid.localeCompare(b.printing_uuid)
  );
}
