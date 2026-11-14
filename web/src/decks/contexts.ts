import type { QueryClient } from "@tanstack/react-query";
import { api, ApiError, type DTO } from "../api/client";
export async function get_contexts(
  client: QueryClient,
  deck_id: string,
  uuids: string[],
) {
  const unique = [...new Set(uuids)],
    missing = unique.filter(
      (uuid) =>
        !client.getQueryData(["deck_context", deck_id, "printing", uuid]),
    );
  for (let offset = 0; offset < missing.length; offset += 200) {
    const batch = missing.slice(offset, offset + 200),
      response = await api.post(
        "/api/v1/decks/{deck_id}/card_contexts",
        { printing_uuids: batch },
        { path: { deck_id } },
      );
    if (
      response.items.length !== batch.length ||
      response.items.some(
        (item, index) =>
          item.requested_printing_uuid !== batch[index] ||
          item.printing_uuid !== item.card.printing_uuid ||
          item.oracle_id !== item.card.oracle_id,
      )
    )
      throw new ApiError(
        "invalid_response",
        "卡牌上下文不完整，未修改当前卡组。",
      );
    response.items.forEach((item) => {
      client.setQueryData(
        ["deck_context", deck_id, "printing", item.requested_printing_uuid],
        item,
      );
      client.setQueryData(
        ["deck_context", deck_id, "printing", item.printing_uuid],
        item,
      );
      client.setQueryData(
        ["deck_context", deck_id, "oracle", item.oracle_id],
        item,
      );
    });
  }
  return new Map(
    unique.map((uuid) => [
      uuid,
      client.getQueryData<DTO<"DeckContextResponse">>([
        "deck_context",
        deck_id,
        "printing",
        uuid,
      ])!,
    ]),
  );
}
