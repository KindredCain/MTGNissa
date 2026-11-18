import type { QueryClient } from "@tanstack/react-query";
import { api, ApiError, type DTO } from "../api/client";
import { createStore } from "zustand/vanilla";

type EditingTask = { kind: "add" | "save" | "parse"; key: string; id: symbol };

export class EditingSession {
  readonly id = crypto.randomUUID();
  private active = false;
  readonly tasks = createStore(() => ({ items: [] as EditingTask[] }));

  get has_tasks() {
    return this.tasks.getState().items.length > 0;
  }

  async run_task<T>(
    kind: EditingTask["kind"],
    key: string,
    work: () => Promise<T>,
  ): Promise<T | undefined> {
    this.assert_active();
    const tasks = this.tasks.getState().items;
    if (
      tasks.some(
        (task) => kind !== "add" || task.kind !== "add" || task.key === key,
      )
    )
      return;
    const id = Symbol();
    this.tasks.setState({ items: [...tasks, { kind, key, id }] });
    try {
      return await this.run(work);
    } finally {
      this.tasks.setState((state) => ({
        items: state.items.filter((task) => task.id !== id),
      }));
    }
  }

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
