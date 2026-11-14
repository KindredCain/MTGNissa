import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  useBlocker,
  useBeforeUnload,
  useNavigate,
  useParams,
} from "react-router";
import { useStore } from "zustand";
import { z } from "zod";
import { api, ApiError } from "../api/client";
import { Board } from "../decks/Board";
import { AddCards } from "../decks/AddCards";
import { create_editor } from "../decks/store";
import {
  context_card,
  deck_save,
  section_types,
  type Deck,
} from "../decks/model";
import { get_contexts } from "../decks/contexts";
import { Modal } from "../ui/feedback";
import { Skeleton } from "../ui/controls";
import styles from "../decks/decks.module.css";
import { Bookmark } from "../decks/Bookmark";
const Statistics = lazy(() => import("../decks/Statistics"));
const name_schema = z
  .string()
  .trim()
  .min(1)
  .refine((value) => Array.from(value).length <= 255)
  .refine(
    (value) =>
      !value.includes(String.fromCharCode(0)) &&
      !/[\r\n\u0085\u2028\u2029]/u.test(value),
  );
export function Component() {
  const { deck_id = "" } = useParams(),
    query = useQuery({
      queryKey: ["decks", "detail", deck_id],
      queryFn: ({ signal }) =>
        api.get("/api/v1/decks/{deck_id}", undefined, {
          path: { deck_id },
          signal,
        }),
      refetchOnMount: "always",
    });
  return query.data && !query.isFetching ? (
    <Editor key={deck_id} initial={query.data} />
  ) : (
    <main className={styles.dark}>
      {(query.isPending || query.isFetching) && <Skeleton />}
    </main>
  );
}
function Editor({ initial }: { initial: Deck }) {
  const [editor] = useState(() => create_editor(initial)),
    state = useStore(editor),
    [source, setSource] = useState(initial.source_text),
    [expanded, setExpanded] = useState(true),
    [add, setAdd] = useState(false),
    [stats, setStats] = useState(false),
    [cover, setCover] = useState(false);
  const client = useQueryClient(),
    navigate = useNavigate(),
    saving = useRef(false);
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      state.dirty &&
      !saving.current &&
      (currentLocation.pathname !== nextLocation.pathname ||
        currentLocation.search !== nextLocation.search),
  );
  useBeforeUnload((event) => {
    if (state.dirty) {
      event.preventDefault();
      event.returnValue = "";
    }
  });
  useEffect(() => {
    for (const section of initial.sections)
      for (const card of section.cards) {
        client.setQueryData(
          ["deck_context", initial.deck_id, "printing", card.printing_uuid],
          card,
        );
        client.setQueryData(
          ["deck_context", initial.deck_id, "oracle", card.oracle_id],
          card,
        );
      }
  }, [client, initial]);
  useEffect(() => {
    if (!cover) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") setCover(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [cover]);
  const save = useMutation({
    mutationFn: async () => {
      const name = name_schema.safeParse(state.deck.name);
      if (!name.success)
        throw new ApiError(
          "validation_failed",
          "卡组名称必须为 1–255 个字符，且不能包含换行。",
        );
      return api.post(
        "/api/v1/decks/{deck_id}/save",
        { ...deck_save(state.deck), name: name.data },
        { path: { deck_id: initial.deck_id } },
      );
    },
    onSuccess: (deck) => {
      saving.current = true;
      state.saved();
      client.setQueryData(["decks", "detail", initial.deck_id], deck);
      void client.invalidateQueries({
        queryKey: ["decks", "list"],
        refetchType: "none",
      });
      void client.invalidateQueries({
        queryKey: ["operation_logs"],
        refetchType: "none",
      });
      navigate(`/decks/${encodeURIComponent(initial.deck_id)}`);
    },
  });
  const parse = useMutation({
    mutationFn: async () => {
      if (
        new TextEncoder().encode(source).length > 786432 ||
        source.split(/\r\n|\r|\n/).length > 1000
      )
        throw new ApiError(
          "validation_failed",
          "卡组代码不能超过 768 KiB 或 1,000 行。",
        );
      const parsed = await api.post(
        "/api/v1/decks/{deck_id}/parse_code",
        { source_text: source },
        { path: { deck_id: initial.deck_id } },
      );
      const contexts = await get_contexts(
        client,
        initial.deck_id,
        parsed.sections.flatMap((section) =>
          section.cards.map((card) => card.printing_uuid),
        ),
      );
      const sections = section_types.map((type) => ({
        type,
        quantity: 0,
        cards: (
          parsed.sections.find((section) => section.type === type)?.cards ?? []
        ).map((card) => {
          const context = contexts.get(card.printing_uuid);
          if (!context)
            throw new ApiError(
              "invalid_response",
              "卡牌上下文不完整，未修改当前卡组。",
            );
          if (type === "commander" && !context.statistics.can_be_commander)
            throw new ApiError(
              "validation_failed",
              "指挥官必须为传奇生物、传奇鹏洛客或传奇载具。",
            );
          return context_card(context, card.quantity);
        }),
      }));
      return { name: parsed.name, sections };
    },
    onSuccess: (result) => {
      editor.getState().replace(result.name, result.sections);
    },
  });
  const locked = save.isPending || parse.isPending;
  return (
    <main className={styles.dark}>
      <fieldset
        disabled={locked}
        style={{ border: 0, padding: 0, margin: 0, minWidth: 0 }}
      >
        <header className={styles.header}>
          <div>
            <input
              className={styles.nameInput}
              aria-label="卡组名称"
              value={state.deck.name}
              onChange={(event) => state.name(event.target.value)}
            />
            <p>
              共 {state.deck.summary.counted_total} 张 · 收集率{" "}
              {Math.round(state.deck.summary.collection_rate * 100)}%
            </p>
          </div>
          <div className={styles.headerActions}>
            <button onClick={() => setAdd(true)}>增加卡牌</button>
            <button
              className={cover ? styles.coverMode : ""}
              disabled={
                !state.deck.sections.some((section) =>
                  section.cards.some((card) => card.quantity > 0),
                )
              }
              aria-pressed={cover}
              onClick={() => setCover((value) => !value)}
            >
              选择封面
            </button>
            <button onClick={() => save.mutate()}>保存卡组</button>
          </div>
        </header>
        <section className={styles.import}>
          <button
            className={styles.importHeader}
            aria-expanded={expanded}
            onClick={() => setExpanded((value) => !value)}
          >
            卡组代码<span>{expanded ? "⌃" : "⌄"}</span>
          </button>
          {expanded && (
            <div className={styles.importBody}>
              <textarea
                aria-label="卡组代码"
                placeholder="在此粘贴卡组代码…"
                value={source}
                onChange={(event) => setSource(event.target.value)}
              />
              <button
                className={styles.importRead}
                onClick={() => parse.mutate()}
              >
                读取卡组配置
              </button>
            </div>
          )}
        </section>
        <Board
          deck={state.deck}
          editor={locked ? undefined : editor}
          cover_mode={cover}
          on_cover={() => setCover(false)}
        />
      </fieldset>
      <Bookmark
        top={expanded ? 660 : 356}
        disabled={locked}
        onClick={() => setStats(true)}
      />
      {stats && (
        <Suspense>
          <Statistics
            deck={state.deck}
            close={() => setStats(false)}
            bookmark_top={expanded ? 660 : 356}
          />
        </Suspense>
      )}
      {add && (
        <AddCards
          deck_id={initial.deck_id}
          editor={editor}
          close={() => setAdd(false)}
        />
      )}
      <Modal open={blocker.state === "blocked"} title="放弃尚未保存的修改？">
        <p>当前卡组还有未保存的修改。</p>
        <div className={styles.confirmActions}>
          <button onClick={() => blocker.reset?.()}>继续编辑</button>
          <button className={styles.danger} onClick={() => blocker.proceed?.()}>
            放弃修改并离开
          </button>
        </div>
      </Modal>
    </main>
  );
}
