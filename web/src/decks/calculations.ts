import { color_names, type_names } from "../shared/display-names";
import {
  colors,
  palette,
  counted_cards,
  type_order,
  type Deck,
  type DeckCard,
} from "./model";
export type Grouping = "type" | "color" | "mana" | "tag" | "produced";
export type Group = {
  key: string;
  name: string;
  color: string;
  cards: DeckCard[];
  quantity: number;
};
export function cost_points(cost: string | null) {
  const result = Object.fromEntries(
    colors.map((color) => [color, 0]),
  ) as Record<(typeof colors)[number], number>;
  for (const token of cost?.match(/\{[^}]+\}/g) ?? [])
    for (const symbol of token.slice(1, -1).split("/"))
      if (colors.includes(symbol as (typeof colors)[number]))
        result[symbol as (typeof colors)[number]]++;
  return result;
}
export function group_cards(cards: DeckCard[], by: Grouping): Group[] {
  const groups = new Map<string, Group>();
  function ensure(key: string, name: string, color = "#FA890D") {
    let group = groups.get(key);
    if (!group) {
      group = { key, name, color, cards: [], quantity: 0 };
      groups.set(key, group);
    }
    return group;
  }
  if (by === "color" || by === "produced")
    colors.forEach((color, index) =>
      ensure(color, color_names[color], palette[index]),
    );
  if (by === "produced") ensure("none", "不产费");
  if (by === "mana")
    for (let index = 0; index <= 8; index++)
      ensure(String(index), index === 8 ? "8+" : String(index));
  for (const card of cards) {
    let keys: string[] = [];
    if (by === "type")
      keys = type_order.filter((type) =>
        card.statistics.type_groups.includes(type),
      );
    if (by === "tag") keys = [...new Set(card.statistics.oracle_tags)];
    if (by === "mana")
      keys = [String(Math.min(8, Math.floor(card.statistics.cmc)))];
    if (by === "produced")
      keys = card.statistics.produced_mana.length
        ? [...new Set(card.statistics.produced_mana)]
        : ["none"];
    if (by === "color")
      keys = colors.filter((color) =>
        color === "C"
          ? cost_points(card.statistics.mana_cost).C > 0
          : card.statistics.colors.includes(color),
      );
    for (const key of keys) {
      const group = ensure(
        key,
        by === "type" ? type_names[key as keyof typeof type_names] : key,
      );
      group.cards.push(card);
      group.quantity += card.quantity;
    }
  }
  const result = [...groups.values()];
  if (by === "tag")
    result.sort(
      (a, b) => b.quantity - a.quantity || a.name.localeCompare(b.name, "en"),
    );
  if (by === "type")
    result.sort(
      (a, b) =>
        type_order.indexOf(a.key as keyof typeof type_names) -
        type_order.indexOf(b.key as keyof typeof type_names),
    );
  return result;
}
export function curve(cards: DeckCard[]) {
  const buckets = Array<number>(9).fill(0);
  for (const card of cards)
    if (!card.statistics.is_land)
      buckets[Math.min(8, Math.floor(card.statistics.cmc))] += card.quantity;
  return buckets;
}
export function overview(deck: Deck) {
  const cards = counted_cards(deck.sections),
    nonlands = cards.filter((card) => !card.statistics.is_land),
    quantity = cards.reduce((sum, card) => sum + card.quantity, 0),
    nonland_total = nonlands.reduce((sum, card) => sum + card.quantity, 0);
  const cost = colors.map(() => 0),
    produced = colors.map(() => 0),
    cost_cards = colors.map(() => 0),
    produced_cards = colors.map(() => 0);
  for (const card of cards) {
    const points = cost_points(card.statistics.mana_cost);
    colors.forEach((color, index) => {
      cost[index] += points[color] * card.quantity;
      if (points[color]) cost_cards[index] += card.quantity;
      if (card.statistics.produced_mana.includes(color)) {
        produced[index] += card.quantity;
        produced_cards[index] += card.quantity;
      }
    });
  }
  return {
    cards,
    quantity,
    lands: quantity - nonland_total,
    average: nonland_total
      ? nonlands.reduce(
          (sum, card) => sum + card.statistics.cmc * card.quantity,
          0,
        ) / nonland_total
      : 0,
    cost,
    produced,
    cost_cards,
    produced_cards,
  };
}
// Hypergeometric survival probability P(X >= at_least), without replacement.
export function probability(
  population: number,
  successes: number,
  draws: number,
  at_least: number,
) {
  draws = Math.max(0, Math.min(population, Math.floor(draws)));
  at_least = Math.floor(at_least);
  if (at_least <= 0) return 1;
  const low = Math.max(0, draws - (population - successes)),
    high = Math.min(draws, successes);
  if (at_least <= low) return 1;
  if (at_least > high || !population) return 0;
  // Normalize recurrence weights around the mode, avoiding factorial overflow
  // and work proportional to a potentially huge draw count.
  const mode = Math.max(
    low,
    Math.min(
      high,
      Math.floor(((draws + 1) * (successes + 1)) / (population + 2)),
    ),
  );
  let total = 1,
    tail = mode >= at_least ? 1 : 0,
    weight = 1;
  for (let k = mode; k > low; k--) {
    weight *=
      ((k / (successes - k + 1)) * (population - successes - draws + k)) /
      (draws - k + 1);
    total += weight;
    if (k - 1 >= at_least) tail += weight;
    if (weight < total * 1e-16) break;
  }
  weight = 1;
  for (let k = mode; k < high; k++) {
    weight *=
      (((successes - k) / (k + 1)) * (draws - k)) /
      (population - successes - draws + k + 1);
    total += weight;
    if (k + 1 >= at_least) tail += weight;
    if (weight < total * 1e-16) break;
  }
  return Math.max(0, Math.min(1, tail / total));
}
