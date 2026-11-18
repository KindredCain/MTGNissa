import { describe, expect, it } from "vitest";
import { context, deck, uuid } from "../test/fixtures";
import { create_editor } from "./store";
import { context_card, deck_save, with_summary } from "./model";
import {
  cost_points,
  group_cards,
  overview,
  probability,
} from "./calculations";
describe("unsaved deck editing", () => {
  it("moves one or all copies, merges by oracle and keeps imported text independent", () => {
    const initial = deck(),
      editor = create_editor(initial),
      id = uuid(1001);
    editor.getState().move("mainboard", "sideboard", id, 1);
    expect(editor.getState().deck.sections[1].quantity).toBe(1);
    expect(editor.getState().deck.sections[2].quantity).toBe(1);
    expect(editor.getState().deck.summary.counted_total).toBe(2);
    expect(editor.getState().deck.source_text).toBe(initial.source_text);
    editor.getState().move("sideboard", "mainboard", id, 1);
    expect(editor.getState().deck.sections[1].cards).toHaveLength(1);
    expect(editor.getState().dirty).toBe(false);
  });
  it("rejects ineligible commander moves without partially changing state", () => {
    const initial = deck();
    initial.sections[1].cards[0].statistics.can_be_commander = false;
    const editor = create_editor(initial),
      before = editor.getState().deck;
    editor.getState().move("mainboard", "commander", uuid(1001), 2);
    expect(editor.getState().deck).toBe(before);
  });
  it("counts the same oracle across main and side only once for ownership", () => {
    const initial = deck();
    initial.sections[2].cards = [context_card(context(), 2)];
    const result = with_summary(initial);
    expect(result.summary.counted_total).toBe(4);
    expect(result.summary.collection_rate).toBe(0.5);
    expect(result.summary.shortage).toEqual({
      distinct_cards: 1,
      missing_quantity: 2,
    });
    expect(result.sections[1].cards[0].exception?.type).toBe("shortage");
    expect(result.sections[2].cards[0].exception?.type).toBe("shortage");
  });
  it("retains a zero-quantity cover until removal and normalizes the save payload", () => {
    const editor = create_editor(deck());
    editor.getState().quantity("mainboard", uuid(1001), -2);
    expect(editor.getState().deck.cover_printing_uuid).toBe(uuid(1));
    const payload = deck_save(editor.getState().deck);
    expect(payload.cover_printing_uuid).toBeNull();
    expect(payload.sections[1].cards).toEqual([]);
    expect(JSON.stringify(payload)).not.toContain("oracle_id");
    expect(payload).not.toHaveProperty("source_text");
    editor.getState().quantity("mainboard", uuid(1001), 1);
    expect(editor.getState().serialize().cover_printing_uuid).toBe(uuid(1));
    editor.getState().quantity("mainboard", uuid(1001), -1);
    editor.getState().remove_zero();
    expect(editor.getState().deck.cover_printing_uuid).toBeNull();
  });
  it("retains the selected cover while its oracle remains in any section", () => {
    const initial = deck();
    const other_context = context(2);
    initial.sections[3].cards = [
      context_card(
        {
          ...other_context,
          oracle_id: uuid(1001),
          card: { ...other_context.card, oracle_id: uuid(1001) },
        },
        1,
      ),
    ];
    const editor = create_editor(initial);
    editor.getState().quantity("mainboard", uuid(1001), -2);
    editor.getState().remove_zero();
    editor.getState().name("仍保留封面");
    expect(editor.getState().deck.cover_printing_uuid).toBe(uuid(1));
    expect(editor.getState().serialize().cover_printing_uuid).toBe(uuid(1));
    editor.getState().quantity("maybeboard", uuid(1001), -1);
    expect(editor.getState().deck.cover_printing_uuid).toBe(uuid(1));
    expect(editor.getState().serialize().cover_printing_uuid).toBeNull();
    editor.getState().remove_zero();
    expect(editor.getState().deck.cover_printing_uuid).toBeNull();
  });
  it("preserves the current name when parsing returns no name", () => {
    const editor = create_editor(deck());
    editor.getState().name("编辑中的名称");
    editor.getState().replace(null, deck().sections);
    expect(editor.getState().deck.name).toBe("编辑中的名称");
  });
});
describe("statistics", () => {
  it("distinguishes explicit colorless cost from generic mana", () => {
    expect(cost_points("{3}{X}{W/U}{W/P}{2/W}{C}")).toEqual({
      W: 3,
      U: 1,
      B: 0,
      R: 0,
      G: 0,
      C: 1,
    });
    const c = context_card(context(), 2);
    c.statistics.colors = [];
    c.statistics.mana_cost = "{3}";
    expect(
      group_cards([c], "color").every((group) => group.quantity === 0),
    ).toBe(true);
  });
  it("excludes lands from the average and sideboard from statistics", () => {
    const d = deck();
    d.sections[2].cards = [context_card(context(2), 20)];
    d.sections[1].cards.push(
      context_card(
        context(3, {
          statistics: { ...context().statistics, is_land: true, cmc: 0 },
        }),
        3,
      ),
    );
    expect(overview(d).average).toBe(1);
    expect(overview(d).lands).toBe(3);
    expect(overview(d).quantity).toBe(5);
  });
  it("calculates at-least hypergeometric probabilities and impossible draws", () => {
    expect(probability(60, 4, 7, 1)).toBeCloseTo(0.3994996257, 8);
    expect(probability(60, 0, 7, 1)).toBe(0);
    expect(probability(60, 60, 7, 7)).toBe(1);
    expect(probability(0, 0, 7, 1)).toBe(0);
    expect(probability(1_000_000_000, 10, 100_000_000, 1)).toBeCloseTo(
      1 - Math.pow(0.9, 10),
      7,
    );
  });
});
