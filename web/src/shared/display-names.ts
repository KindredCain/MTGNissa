import type { DTO } from "../api/client";

// Display text only. Option ranges, ordering and behavior belong to their features.
const language_names: Readonly<Record<string, string>> = {
  en: "英文",
  es: "西语",
  fr: "法语",
  de: "德语",
  it: "意语",
  pt: "葡语",
  ja: "日文",
  ko: "韩文",
  ru: "俄语",
  zhs: "简中",
  zht: "繁中",
  he: "希伯",
  la: "拉丁",
  grc: "古希",
  ar: "阿语",
  sa: "梵文",
  ph: "非瑞",
  qya: "昆雅",
  dw: "矮人",
};

export const color_names = {
  W: "白色",
  U: "蓝色",
  B: "黑色",
  R: "红色",
  G: "绿色",
  C: "无色",
  multicolor: "多色",
  colorless: "无色",
} as const satisfies Readonly<Record<DTO<"Color"> | DTO<"ColorGroup">, string>>;

export const rarity_names = {
  common: "铁",
  uncommon: "银",
  rare: "金",
  mythic: "秘稀",
  special: "特殊",
  bonus: "额外",
} as const satisfies Readonly<Record<DTO<"Rarity">, string>>;

export const finish_names = {
  normal: "普通",
  foil: "闪卡",
} as const satisfies Readonly<
  Record<NonNullable<DTO<"OperationCardResponse">["finish"]>, string>
>;

export const section_names = {
  commander: "指挥官",
  mainboard: "主牌",
  sideboard: "备牌",
  maybeboard: "备选",
} as const satisfies Readonly<Record<DTO<"SectionType">, string>>;

export const type_names = {
  artifact: "神器",
  creature: "生物",
  enchantment: "结界",
  instant: "瞬间",
  sorcery: "法术",
  planeswalker: "鹏洛客",
  land: "地",
  other: "其他",
} as const satisfies Readonly<Record<DTO<"TypeGroup">, string>>;

export const action_names = {
  add_card: "增加卡牌",
  remove_card: "减少卡牌",
  add_wishlist: "加入愿望单",
  remove_wishlist: "移出愿望单",
  create_deck: "创建卡组",
  delete_deck: "删除卡组",
  copy_deck: "复制卡组",
  modify_deck: "修改卡组",
} as const satisfies Readonly<Record<DTO<"ActionType">, string>>;

export const exception_names = {
  shortage: "数量不足",
  deck_usage: "卡组占用",
} as const satisfies Readonly<
  Record<DTO<"DeckExceptionResponse">["type"], string>
>;

export const set_sort_names = {
  released_desc: "发行日期 ↓",
  released_asc: "发行日期 ↑",
  name_asc: "名称 ↑",
  name_desc: "名称 ↓",
  code_asc: "简写 ↑",
  code_desc: "简写 ↓",
  card_count_desc: "卡牌数量 ↓",
  card_count_asc: "卡牌数量 ↑",
} as const;

export const card_sort_names = {
  collector_number_desc: "收藏编号 ↓",
  collector_number_asc: "收藏编号 ↑",
  mana_value_desc: "法术力值 ↓",
  mana_value_asc: "法术力值 ↑",
  name_desc: "名称 ↓",
  name_asc: "名称 ↑",
} as const;

export const personal_sort_names = {
  collection: {
    set_desc: "系列发行 ↓",
    color: "颜色",
    mana_value_asc: "法术力值 ↑",
    name_asc: "名称 ↑",
  },
  wishlist: {
    set_desc: "系列发行 ↓",
    color: "颜色",
    mana_value_asc: "法术力值 ↑",
    name_asc: "名称 ↑",
  },
} as const;

export const deck_sort_names = {
  updated_desc: "更新时间 ↓",
  created_desc: "创建时间 ↓",
  name_asc: "名称 ↑",
  card_count_desc: "卡牌数量 ↓",
} as const;

// Names are presentation only; available codes and their order come from the API.
export function language_name(code: string): string {
  return Object.hasOwn(language_names, code) ? language_names[code]! : code;
}
