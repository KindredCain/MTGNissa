import type { DTO } from "../api/client";
import { context_card } from "../decks/model";
export const uuid = (index: number) =>
  `00000000-0000-4000-8000-${String(index).padStart(12, "0")}`;
export function card(
  index = 1,
  overrides: Partial<DTO<"CardSummaryResponse">> = {},
): DTO<"CardSummaryResponse"> {
  return {
    printing_uuid: uuid(index),
    oracle_id: uuid(index + 1000),
    name: `Test Card ${index}`,
    display_name: `测试卡牌 ${index}`,
    set: {
      code: "KLD",
      name: "Kaladesh",
      display_name: "卡拉德许",
      released_at: "2016-09-30",
    },
    collector_number: String(index),
    language: { code: "en", sort_order: 1 },
    released_at: "2016-09-30",
    cmc: index % 8,
    mana_cost: "{1}{G}",
    colors: ["G"],
    rarity: "rare",
    layout: "normal",
    images: {
      front_url: `https://cards.example.invalid/${index}.jpg`,
      back_url: null,
      has_distinct_back: false,
    },
    ...overrides,
  };
}
export function card_face(
  index = 1,
  overrides: Partial<DTO<"CardFaceResponse">> = {},
): DTO<"CardFaceResponse"> {
  const c = card(index);
  return {
    face_index: -1,
    name: c.name,
    display_name: c.display_name,
    flavor_name: null,
    mana_cost: c.mana_cost,
    type_line: "Legendary Creature",
    display_type_line: "传奇生物",
    oracle_text: "Rules",
    display_oracle_text: "测试规则文字。",
    flavor_text: null,
    display_flavor_text: null,
    power: "2",
    toughness: "2",
    loyalty: null,
    image_url: c.images.front_url,
    ...overrides,
  };
}
export function context(
  index = 1,
  overrides: Partial<DTO<"DeckContextResponse">> = {},
): DTO<"DeckContextResponse"> {
  const c = card(index);
  return {
    printing_uuid: c.printing_uuid,
    oracle_id: c.oracle_id!,
    card: c,
    type_group: "creature",
    faces: [card_face(index)],
    owned_quantity: 2,
    other_decks_quantity: 0,
    other_decks_count: 0,
    statistics: {
      cmc: c.cmc,
      mana_cost: c.mana_cost,
      colors: c.colors,
      produced_mana: ["G"],
      type_groups: ["creature"],
      oracle_tags: ["Ramp"],
      is_land: false,
      can_be_commander: true,
    },
    ...overrides,
  };
}
export function deck(): DTO<"DeckDetailResponse"> {
  return {
    deck_id: "1",
    name: "测试卡组",
    source_text: "About\nName 测试卡组\n\nDeck\n2 Test Card 1",
    cover_printing_uuid: uuid(1),
    created_at: "2026-10-09T04:00:00Z",
    updated_at: "2026-10-09T04:00:00Z",
    sections: [
      { type: "commander", quantity: 0, cards: [] },
      { type: "mainboard", quantity: 2, cards: [context_card(context(), 2)] },
      { type: "sideboard", quantity: 0, cards: [] },
      { type: "maybeboard", quantity: 0, cards: [] },
    ],
    summary: {
      counted_total: 2,
      collection_rate: 1,
      shortage: { distinct_cards: 0, missing_quantity: 0 },
      deck_usage: { distinct_cards: 0, missing_quantity: 0 },
    },
  };
}
export function detail(): DTO<"CardDetailResponse"> {
  const c = card(1, {
    images: {
      front_url: "https://cards.example.invalid/1.jpg",
      back_url: "https://cards.example.invalid/back.jpg",
      has_distinct_back: true,
    },
  });
  return {
    card: c,
    faces: ([0, 1] as const).map((index) => ({
      face_index: index,
      name: index ? "Back" : "Front",
      display_name: index ? "卡牌背面" : "卡牌正面",
      flavor_name: null,
      mana_cost: index ? "{R}" : "{1}{G}",
      type_line: "Creature",
      display_type_line: "生物",
      oracle_text: "Rules",
      display_oracle_text: "测试规则。",
      flavor_text: null,
      display_flavor_text: null,
      power: "2",
      toughness: "2",
      loyalty: null,
      image_url: index ? c.images.back_url : c.images.front_url,
    })),
    collection: { normal_quantity: 2, foil_quantity: 0 },
    logical_collection: {
      normal_quantity: 2,
      foil_quantity: 0,
      total_quantity: 2,
    },
    wishlist: { included: true },
    rulings: [{ published_at: "2026-10-09", text: "测试释疑。" }],
    oracle_tags: [{ tag_id: "1", query: 'otag:"mana rock"' }],
    languages: [
      {
        printing_uuid: c.printing_uuid,
        oracle_id: c.oracle_id,
        language: c.language,
        normal_quantity: 2,
        foil_quantity: 0,
      },
    ],
    other_versions: [],
    deck_usage: {
      total_decks: 1,
      decks: [
        {
          deck_id: "1",
          name: "测试卡组",
          quantity: 2,
          sections: ["mainboard"],
        },
      ],
    },
  };
}
export const options: DTO<"CollectionOptionsResponse"> = {
  sets: [{ code: "KLD", display_name: "卡拉德许" }],
  languages: [{ code: "en", sort_order: 1 }],
  colors: ["W", "U", "B", "R", "G", "multicolor", "colorless"],
  mana_values: ["0", "1", "2", "3", "4", "5", "6", "7", "8_plus"],
  rarities: ["common", "uncommon", "rare", "mythic", "special", "bonus"],
};
export const set_item: DTO<"SetListItemResponse"> = {
  code: "KLD",
  name: "Kaladesh",
  display_name: "卡拉德许",
  set_type: "expansion",
  card_count: 42,
  released_at: "2016-09-30",
  languages: [
    { code: "en", sort_order: 1, available: true },
    { code: "ja", sort_order: 2, available: false },
  ],
  default_language: "en",
  parent_code: null,
  depth: 0,
};
