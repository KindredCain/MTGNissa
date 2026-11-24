import { createStore } from "zustand/vanilla";
import {
  deck_save,
  with_summary,
  type Deck,
  type DeckCard,
  type SectionType,
} from "./model";
type State = {
  deck: Deck;
  dirty: boolean;
  baseline: string;
  cover_oracle_id?: string;
  name: (name: string) => void;
  cover: (printing_uuid: string | null) => void;
  quantity: (
    section: SectionType,
    oracle_id: string,
    delta: number,
  ) => string | undefined;
  remove_zero: () => void;
  add: (card: DeckCard, quantity: number) => string | undefined;
  move: (
    from: SectionType,
    to: SectionType,
    oracle_id: string,
    quantity: number,
  ) => string | undefined;
  replace: (name: string | null, sections: Deck["sections"]) => void;
  saved: () => void;
  serialize: () => ReturnType<typeof deck_save>;
};
export function create_editor(initial: Deck) {
  return createStore<State>((set, get) => {
    let cover_oracle_id = initial.sections
      .flatMap((section) => section.cards)
      .find(
        (card) => card.printing_uuid === initial.cover_printing_uuid,
      )?.oracle_id;
    function commit(deck: Deck) {
      const all = deck.sections.flatMap((section) => section.cards);
      if (deck.cover_printing_uuid !== get().deck.cover_printing_uuid)
        cover_oracle_id = all.find(
          (card) => card.printing_uuid === deck.cover_printing_uuid,
        )?.oracle_id;
      if (
        deck.cover_printing_uuid &&
        !all.some((card) => card.oracle_id === cover_oracle_id)
      )
        deck = { ...deck, cover_printing_uuid: null };
      const next = with_summary(deck);
      set({
        deck: next,
        cover_oracle_id: next.cover_printing_uuid ? cover_oracle_id : undefined,
        dirty:
          JSON.stringify(deck_save(next, cover_oracle_id)) !== get().baseline,
      });
    }
    function alter(
      section: SectionType,
      oracle_id: string,
      delta: number,
      deck = get().deck,
    ) {
      return {
        ...deck,
        sections: deck.sections.map((part) =>
          part.type === section
            ? {
                ...part,
                cards: part.cards.map((card) =>
                  card.oracle_id === oracle_id
                    ? { ...card, quantity: card.quantity + delta }
                    : card,
                ),
              }
            : part,
        ),
      };
    }
    return {
      deck: initial,
      dirty: false,
      baseline: JSON.stringify(deck_save(initial)),
      cover_oracle_id,
      serialize: () => deck_save(get().deck, cover_oracle_id),
      name: (name) => commit({ ...get().deck, name }),
      cover: (printing_uuid) =>
        commit({ ...get().deck, cover_printing_uuid: printing_uuid }),
      quantity: (section, oracle_id, delta) => {
        const card = get()
          .deck.sections.find((part) => part.type === section)
          ?.cards.find((card) => card.oracle_id === oracle_id);
        if (!card || card.quantity + delta < 0) return;
        if (card.quantity + delta > 4294967295)
          return "单张卡牌数量不能超过 4,294,967,295。";
        commit(alter(section, oracle_id, delta));
      },
      remove_zero: () => {
        if (
          !get().deck.sections.some((section) =>
            section.cards.some((card) => card.quantity === 0),
          )
        )
          return;
        commit({
          ...get().deck,
          sections: get().deck.sections.map((section) => ({
            ...section,
            cards: section.cards.filter((card) => card.quantity > 0),
          })),
        });
      },
      add: (card, quantity) => {
        const deck = get().deck,
          section = deck.sections.find((part) => part.type === "mainboard")!,
          existing = section.cards.find(
            (item) => item.oracle_id === card.oracle_id,
          );
        if (existing)
          return get().quantity("mainboard", card.oracle_id, quantity);
        else
          commit({
            ...deck,
            sections: deck.sections.map((part) =>
              part.type === "mainboard"
                ? { ...part, cards: [...part.cards, { ...card, quantity }] }
                : part,
            ),
          });
      },
      move: (from, to, oracle_id, quantity) => {
        if (from === to) return;
        const deck = get().deck,
          card = deck.sections
            .find((section) => section.type === from)
            ?.cards.find((card) => card.oracle_id === oracle_id);
        if (
          !card ||
          !quantity ||
          card.quantity < quantity ||
          (to === "commander" && !card.statistics.can_be_commander)
        )
          return;
        let next = alter(from, oracle_id, -quantity);
        const target = next.sections.find((section) => section.type === to)!,
          existing = target.cards.find((item) => item.oracle_id === oracle_id);
        if ((existing?.quantity ?? 0) + quantity > 4294967295)
          return "单张卡牌数量不能超过 4,294,967,295。";
        if (existing) next = alter(to, oracle_id, quantity, next);
        else
          next = {
            ...next,
            sections: next.sections.map((section) =>
              section.type === to
                ? {
                    ...section,
                    cards: [...section.cards, { ...card, quantity }],
                  }
                : section,
            ),
          };
        commit({
          ...next,
          sections: next.sections.map((section) => ({
            ...section,
            cards: section.cards.filter((card) => card.quantity > 0),
          })),
        });
      },
      replace: (name, sections) =>
        commit({ ...get().deck, name: name ?? get().deck.name, sections }),
      saved: () =>
        set({ dirty: false, baseline: JSON.stringify(get().serialize()) }),
    };
  });
}
export type EditorStore = ReturnType<typeof create_editor>;
