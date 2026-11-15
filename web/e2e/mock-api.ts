import { expect, type Page } from "@playwright/test";
import {
  card,
  context,
  deck,
  detail,
  options,
  set_item,
  uuid,
} from "../src/test/fixtures";
import type { DTO } from "../src/api/client";
import path from "node:path";
export async function mock_api(page: Page) {
  const state = {
    normal: 2,
    foil: 0,
    included: true,
    fail_context: false,
    fail_adjustment: false,
    fail_lists: false,
    delay_adjustment: 0,
    cards_requests: 0,
    adjustments: 0,
    save_body: undefined as DTO<"DeckSaveRequest"> | undefined,
    deck: deck(),
  };
  await page.route("https://cards.example.invalid/**", (route) =>
    route.fulfill({
      path: path.resolve("../design/assets/card-samples/nissa-kld-163-ja.jpg"),
      contentType: "image/jpeg",
    }),
  );
  await page.route("https://svgs.scryfall.io/**", (route) =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><circle cx="10" cy="10" r="10" fill="#93cba4"/></svg>',
    }),
  );
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url()),
      method = route.request().method(),
      endpoint = url.pathname.replace("/api/v1", "");
    let data: unknown = {},
      status = 200;
    if (
      state.fail_lists &&
      url.searchParams.has("q") &&
      ["/sets", "/cards", "/collection", "/wishlist", "/decks"].includes(
        endpoint,
      )
    ) {
      await route.fulfill({
        status: 503,
        json: {
          status: 503,
          data: null,
          error: {
            code: "dependency_unavailable",
            message: "database unavailable",
          },
          request_id: "test",
        },
      });
      return;
    }
    if (endpoint === "/app")
      data = { display_name: "Kindred 的牌库", timezone: "Asia/Shanghai" };
    else if (endpoint === "/sets/options")
      data = {
        languages: [
          { code: "en", display_name: "英文", sort_order: 1 },
          { code: "ja", display_name: "日文", sort_order: 2 },
        ],
        set_types: ["expansion", "core"],
      };
    else if (endpoint === "/sets")
      data = {
        items: [
          set_item,
          {
            ...set_item,
            code: "TKLD",
            name: "Kaladesh Tokens",
            display_name: "卡拉德许衍生物",
            set_type: "token",
            languages: [],
            default_language: null,
            parent_code: "KLD",
            depth: 1,
          },
        ],
        total: 1,
        next_cursor: null,
      };
    else if (endpoint === "/cards") {
      state.cards_requests++;
      data = {
        set: card().set,
        language: card().language,
        items: Array.from({ length: 42 }, (_, i) => card(i + 1)),
        total: 42,
      };
    } else if (endpoint === "/tokens")
      data = {
        set: card().set,
        items: [
          {
            token_uuid: uuid(99),
            name: "衍生物",
            collector_number: "1",
            layout: "token",
            images: card().images,
          },
        ],
        total: 1,
      };
    else if (endpoint === "/search")
      data = {
        input: url.searchParams.get("q"),
        mode: "syntax",
        executed_query: url.searchParams.get("q"),
        items: [card()],
        total: 1,
        next_cursor: null,
      };
    else if (
      endpoint === "/collection/options" ||
      endpoint === "/wishlist/options"
    )
      data = options;
    else if (endpoint === "/collection")
      data = {
        items: [
          {
            card: card(),
            group_set: card().set,
            group_summary: {
              total_quantity: state.normal + state.foil,
              distinct_cards: 1,
            },
            normal_quantity: state.normal,
            foil_quantity: state.foil,
            merged_sets: url.searchParams.get("merge_sets") === "true",
            merged_languages:
              url.searchParams.get("merge_languages") !== "false",
          },
        ],
        summary: {
          total_quantity: state.normal + state.foil,
          distinct_cards: 1,
        },
        total: 1,
        next_cursor: null,
      };
    else if (endpoint === "/wishlist")
      data = {
        items: state.included
          ? [{ card: card(), group_set: card().set, included: true }]
          : [],
        total: state.included ? 1 : 0,
        next_cursor: null,
      };
    else if (endpoint.match(/^\/cards\/[^/]+$/))
      data = {
        ...detail(),
        collection: {
          normal_quantity: state.normal,
          foil_quantity: state.foil,
        },
        logical_collection: {
          normal_quantity: state.normal,
          foil_quantity: state.foil,
          total_quantity: state.normal + state.foil,
        },
        wishlist: { included: state.included },
      };
    else if (endpoint.endsWith("/collection_adjustments")) {
      state.adjustments++;
      if (state.delay_adjustment)
        await new Promise((resolve) =>
          setTimeout(resolve, state.delay_adjustment),
        );
      if (state.fail_adjustment) {
        await route.fulfill({
          status: 409,
          json: {
            status: 409,
            data: null,
            error: {
              code: "collection_underflow",
              message: "没有足够的卡牌。",
            },
            request_id: "test",
          },
        });
        return;
      }
      const body = route.request().postDataJSON();
      if (body.finish === "normal") state.normal += body.delta;
      else state.foil += body.delta;
      expect(route.request().headers()["idempotency-key"]).toMatch(
        /^[\da-f-]{36}$/,
      );
      data = {
        printing_uuid: uuid(1),
        oracle_id: uuid(1001),
        group: {
          normal_quantity: state.normal,
          foil_quantity: state.foil,
          total_quantity: state.normal + state.foil,
        },
        collection: {
          total_quantity: state.normal + state.foil,
          distinct_cards: 1,
        },
        set_summary: {
          set_code: "KLD",
          total_quantity: state.normal + state.foil,
          distinct_cards: 1,
        },
      };
    } else if (endpoint.endsWith("/wishlist_state")) {
      state.included = route.request().postDataJSON().included;
      data = {
        printing_uuid: uuid(1),
        oracle_id: uuid(1001),
        included: state.included,
      };
    } else if (endpoint === "/decks" && method === "POST") {
      data = { deck_id: "1", name: "新建卡组" };
      status = 201;
    } else if (endpoint === "/decks")
      data = {
        items: [
          {
            deck_id: "1",
            name: state.deck.name,
            created_at: state.deck.created_at,
            updated_at: state.deck.updated_at,
            cover_card: card(),
            section_counts: {
              commander: 0,
              mainboard: 2,
              sideboard: 0,
              maybeboard: 0,
            },
            counted_total: 2,
            color_distribution: { W: 0, U: 0, B: 0, R: 0, G: 2, C: 0 },
            exception: null,
          },
        ],
        total: 1,
        next_cursor: null,
      };
    else if (endpoint === "/decks/1") data = state.deck;
    else if (endpoint.endsWith("/cards/name_suggestions"))
      data = { items: [{ printed_name: "Test Card 2" }] };
    else if (endpoint.endsWith("/cards/search"))
      data = {
        items: [
          {
            card: card(2),
            faces: context(2).faces,
          },
        ],
        total: 1,
        next_cursor: null,
      };
    else if (endpoint.endsWith("/card_contexts")) {
      if (state.fail_context) {
        await route.fulfill({
          status: 503,
          json: {
            status: 503,
            data: null,
            error: {
              code: "dependency_unavailable",
              message: "database unavailable",
            },
            request_id: "test",
          },
        });
        return;
      }
      const body = route.request().postDataJSON() as DTO<"DeckContextsRequest">;
      data = {
        items: body.printing_uuids.map((id) => ({
          ...context(id === uuid(2) ? 2 : 1),
          requested_printing_uuid: id,
        })),
      };
    } else if (endpoint.endsWith("/parse_code"))
      data = {
        name: null,
        sections: [
          {
            type: "mainboard",
            cards: [
              {
                printing_uuid: uuid(2),
                oracle_id: uuid(1002),
                quantity: 3,
                card: card(2),
              },
            ],
          },
        ],
      };
    else if (endpoint.endsWith("/save")) {
      state.save_body = route.request().postDataJSON();
      state.deck = {
        ...state.deck,
        name: state.save_body!.name,
        cover_printing_uuid: state.save_body!.cover_printing_uuid,
      };
      data = state.deck;
    } else if (endpoint.endsWith("/copies")) {
      data = { deck_id: "1", name: "测试卡组（复制）" };
      status = 201;
    } else if (endpoint.endsWith("/delete"))
      data = { deck_id: "1", deleted: true };
    else if (endpoint === "/operation_logs/summary")
      data = {
        items: [
          { operation_date: "2026-10-09", action_type: "add_card", count: 1 },
        ],
        page: 1,
        page_size: 10,
        total_items: 1,
        total_pages: 1,
      };
    else if (endpoint === "/operation_logs")
      data = {
        items: [
          {
            operation_id: "1",
            kind: "card",
            action_type: "add_card",
            occurred_at: "2026-10-09T04:00:00Z",
            card: {
              printing_uuid: uuid(1),
              name: "历史卡牌名称",
              set_code: "KLD",
              collector_number: "1",
              language: "en",
              language_display_name: "英文",
              finish: "normal",
              quantity_delta: 1,
            },
          },
        ],
        page: 1,
        page_size: 10,
        total_items: 1,
        total_pages: 1,
      };
    else throw Error(`Unhandled mock endpoint ${method} ${endpoint}`);
    await route.fulfill({
      status,
      json: { status, data, error: null, request_id: "browser-test" },
    });
  });
  return state;
}
