import type { QueryClient } from "@tanstack/react-query";
import { api, ApiError, type DTO } from "../api/client";

export class EditingSession {
  readonly id = crypto.randomUUID();
  private active = false;

  activate() {
    this.active = true;
  }

  close() {
    this.active = false;
  }

  get is_active() {
    return this.active;
  }

  assert_active() {
    if (!this.active)
      throw new DOMException("Editing session ended", "AbortError");
  }

  prefix(deck_id: string) {
    return ["deck_context", deck_id, this.id] as const;
  }

  key(deck_id: string, kind: "printing" | "oracle", id: string) {
    return [...this.prefix(deck_id), kind, id] as const;
  }

  async run<T>(work: () => Promise<T>): Promise<T> {
    this.assert_active();
    try {
      const result = await work();
      this.assert_active();
      return result;
    } catch (error) {
      this.assert_active();
      throw error;
    }
  }
}

export async function get_contexts(
  client: QueryClient,
  deck_id: string,
  session: EditingSession,
  uuids: string[],
) {
  session.assert_active();
  const unique = [...new Set(uuids)],
    missing = unique.filter(
      (uuid) => !client.getQueryData(session.key(deck_id, "printing", uuid)),
    );
  for (let offset = 0; offset < missing.length; offset += 200) {
    const batch = missing.slice(offset, offset + 200),
      response = await session.run(() =>
        api.post(
          "/api/v1/decks/{deck_id}/card_contexts",
          { printing_uuids: batch },
          { path: { deck_id } },
        ),
      );
    session.assert_active();
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
        session.key(deck_id, "printing", item.requested_printing_uuid),
        item,
      );
      client.setQueryData(
        session.key(deck_id, "printing", item.printing_uuid),
        item,
      );
      client.setQueryData(session.key(deck_id, "oracle", item.oracle_id), item);
    });
  }
  return new Map(
    unique.map((uuid) => [
      uuid,
      client.getQueryData<DTO<"DeckContextResponse">>(
        session.key(deck_id, "printing", uuid),
      )!,
    ]),
  );
}
