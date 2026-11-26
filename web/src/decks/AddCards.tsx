import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  keepPreviousData,
  infiniteQueryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useStore } from "zustand";
import { api, type DTO } from "../api/client";
import { Modal, validation_error } from "../ui/feedback";
import { LoadMore } from "../ui/controls";
import { context_card } from "./model";
import { get_contexts, type EditingSession } from "./contexts";
import type { EditorStore } from "./store";
import { PreviewBubble } from "./Board";
import styles from "./decks.module.css";
import { useHoverPreview } from "../ui/preview";
const quantity_schema = z.object({
  quantity: z.number().int().min(1).max(4294967295),
});
function candidate_options(
  deck_id: string,
  submitted: { q: string; exact: boolean; revision: number } | null,
) {
  return infiniteQueryOptions({
    queryKey: [
      "deck_candidates",
      deck_id,
      submitted?.q,
      submitted?.exact,
      submitted?.revision,
    ],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks/{deck_id}/cards/search",
        {
          q: submitted?.q,
          exact: submitted?.exact,
          cursor: pageParam ?? undefined,
        },
        { path: { deck_id }, signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function AddCards({
  deck_id,
  session,
  editor,
  close,
}: {
  deck_id: string;
  session: EditingSession;
  editor: EditorStore;
  close: () => void;
}) {
  const client = useQueryClient(),
    search_gate = useRef(false),
    revision = useRef(0),
    close_timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [searching, setSearching] = useState(false);
  const adding = useStore(session.tasks, (state) =>
    state.items.some((task) => task.kind === "add"),
  );
  const [modal_root, setModalRoot] = useState<HTMLDivElement | null>(null);
  const [input, setInput] = useState(""),
    [suggestion, setSuggestion] = useState(""),
    [show, setShow] = useState(false),
    [submitted, setSubmitted] = useState<{
      q: string;
      exact: boolean;
      revision: number;
    } | null>(null),
    [scroll_root, setScrollRoot] = useState<HTMLDivElement | null>(null);
  const {
    value: preview,
    show: show_preview,
    keep: keep_preview,
    leave: hide_preview,
    hide: hide_bubble,
  } = useHoverPreview<{
    item: DTO<"DeckCandidateResponse">;
    anchor: HTMLElement;
  }>();
  const list = useInfiniteQuery({
    queryKey: [
      "deck_candidates",
      deck_id,
      submitted?.q,
      submitted?.exact,
      submitted?.revision,
    ],
    initialPageParam: null as string | null,
    enabled: submitted !== null,
    placeholderData: keepPreviousData,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks/{deck_id}/cards/search",
        {
          q: submitted?.q,
          exact: submitted?.exact,
          cursor: pageParam ?? undefined,
        },
        { path: { deck_id }, signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  useEffect(() => {
    const id = setTimeout(() => setSuggestion(input.trim()), 250);
    return () => clearTimeout(id);
  }, [input]);
  useEffect(() => cancel_suggestion_close, []);
  function cancel_suggestion_close() {
    if (close_timer.current !== null) clearTimeout(close_timer.current);
    close_timer.current = null;
  }
  const enough =
    (suggestion.match(/[A-Za-z]/g)?.length ?? 0) >= 3 ||
    (suggestion.match(/\p{Script=Han}/gu)?.length ?? 0) >= 2;
  const suggestions = useQuery({
    queryKey: ["deck_suggestions", deck_id, suggestion],
    enabled: enough && show,
    queryFn: ({ signal }) =>
      api.get(
        "/api/v1/decks/{deck_id}/cards/name_suggestions",
        { q: suggestion },
        { path: { deck_id }, signal },
      ),
  });
  async function submit(q: string, exact = false) {
    if (search_gate.current) return;
    search_gate.current = true;
    setSearching(true);
    setShow(false);
    const next = { q: q.trim(), exact, revision: ++revision.current };
    try {
      await client.fetchInfiniteQuery(candidate_options(deck_id, next));
      setSubmitted(next);
      scroll_root?.scrollTo({ top: 0 });
    } catch {
      /* QueryCache reports the failure; keep the previous accepted results. */
    } finally {
      search_gate.current = false;
      setSearching(false);
    }
  }
  return (
    <Modal
      open
      title="增加卡牌"
      className={styles.addModal}
      close={() => {
        if (!session.has_tasks) close();
      }}
      close_disabled={adding}
      close_label="×"
      content_ref={setModalRoot}
    >
      <form
        className={styles.addTools}
        onSubmit={(event) => {
          event.preventDefault();
          submit(input);
        }}
      >
        <div
          className={styles.addSearchField}
          onFocus={cancel_suggestion_close}
          onBlur={(event) => {
            if (event.currentTarget.contains(event.relatedTarget)) return;
            cancel_suggestion_close();
            close_timer.current = setTimeout(() => {
              close_timer.current = null;
              setShow(false);
            }, 150);
          }}
        >
          <svg
            className={styles.addSearchIcon}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            aria-hidden="true"
          >
            <circle cx="10.5" cy="10.5" r="6.5" />
            <path d="m16 16 5 5" />
          </svg>
          <input
            value={input}
            maxLength={512}
            placeholder="输入卡牌名称"
            aria-label="搜索加入卡组的卡牌"
            autoComplete="off"
            onChange={(event) => {
              setInput(event.target.value);
              setShow(true);
            }}
            onFocus={() => setShow(true)}
          />
          {show &&
            enough &&
            suggestion === input.trim() &&
            !!suggestions.data?.items.length && (
              <div className={styles.suggestions}>
                {suggestions.data.items.map((item) => (
                  <button
                    type="button"
                    key={item.printed_name}
                    onPointerDown={(event) => event.preventDefault()}
                    onClick={() => {
                      setInput(item.printed_name);
                      submit(item.printed_name, true);
                    }}
                  >
                    {item.printed_name}
                  </button>
                ))}
              </div>
            )}
        </div>
        <button disabled={searching}>搜索</button>
        <span>找到 {list.data?.pages[0].total ?? 0} 张</span>
      </form>
      <div className={styles.results} ref={setScrollRoot}>
        {list.data?.pages
          .flatMap((page) => page.items)
          .map((item) => (
            <Candidate
              key={item.card.oracle_id ?? item.card.printing_uuid}
              item={item}
              deck_id={deck_id}
              session={session}
              editor={editor}
              onPreview={(anchor) => show_preview({ item, anchor })}
              onLeave={hide_preview}
            />
          ))}
        <LoadMore
          failed={list.isFetchNextPageError}
          has_more={!!list.hasNextPage && !list.isPlaceholderData}
          loading={list.isFetching}
          root={scroll_root}
          load={() => {
            void list.fetchNextPage();
          }}
        />
      </div>
      {preview &&
        modal_root &&
        createPortal(
          <PreviewBubble
            variant="image"
            in_modal
            onPointerEnter={keep_preview}
            onPointerLeave={hide_bubble}
            preview={{
              card: candidate_preview(preview.item),
              anchor: preview.anchor,
            }}
          />,
          modal_root,
        )}
    </Modal>
  );
}
function Candidate({
  item,
  deck_id,
  session,
  editor,
  onPreview,
  onLeave,
}: {
  item: DTO<"DeckCandidateResponse">;
  deck_id: string;
  session: EditingSession;
  editor: EditorStore;
  onPreview: (anchor: HTMLElement) => void;
  onLeave: () => void;
}) {
  const client = useQueryClient(),
    gate = useRef(false);
  const task_key = item.card.oracle_id ?? item.card.printing_uuid;
  const pending = useStore(session.tasks, (state) =>
    state.items.some((task) => task.kind === "add" && task.key === task_key),
  );
  const form = useForm({
    defaultValues: { quantity: 1 },
    resolver: zodResolver(quantity_schema),
  });
  const add = useMutation({
    mutationFn: (quantity: number) =>
      session.run_task("add", task_key, async () => {
        const cached = item.card.oracle_id
          ? client.getQueryData<DTO<"DeckContextResponse">>(
              session.key(deck_id, "oracle", item.card.oracle_id),
            )
          : undefined;
        const context =
          cached ??
          (
            await get_contexts(client, deck_id, session, [
              item.card.printing_uuid,
            ])
          ).get(item.card.printing_uuid)!;
        session.assert_active();
        const error = editor
          .getState()
          .add(context_card(context, quantity), quantity);
        if (error) validation_error(error);
      }),
    onSettled: () => {
      gate.current = false;
    },
  });
  return (
    <form
      noValidate
      className={styles.resultRow}
      onPointerEnter={(event) => {
        onPreview(event.currentTarget);
      }}
      onPointerLeave={onLeave}
      onSubmit={(event) => {
        void form.handleSubmit(
          ({ quantity }) => {
            if (gate.current || pending) return;
            gate.current = true;
            add.mutate(quantity);
          },
          () => validation_error("加入数量必须为 1 至 4,294,967,295 的整数。"),
        )(event);
      }}
    >
      <button
        type="button"
        className={styles.resultName}
        onFocus={(event) => {
          onPreview(event.currentTarget);
        }}
        onBlur={onLeave}
        onClick={(event) => {
          onPreview(event.currentTarget);
        }}
      >
        {item.card.display_name}
      </button>
      <input
        aria-label={`${item.card.display_name}加入数量`}
        type="number"
        min={1}
        max={4294967295}
        step={1}
        disabled={pending || add.isPending}
        {...form.register("quantity", { valueAsNumber: true })}
      />
      <button type="submit" disabled={pending || add.isPending}>
        {pending || add.isPending ? "加入中" : "加入"}
      </button>
    </form>
  );
}

function candidate_preview(item: DTO<"DeckCandidateResponse">) {
  return context_card(
    {
      printing_uuid: item.card.printing_uuid,
      oracle_id: item.card.oracle_id ?? "",
      card: item.card,
      type_group: "other",
      faces: item.faces,
      owned_quantity: 0,
      other_decks_quantity: 0,
      other_decks_count: 0,
      statistics: {
        cmc: item.card.cmc,
        mana_cost: item.card.mana_cost,
        colors: item.card.colors,
        produced_mana: [],
        type_groups: [],
        oracle_tags: [],
        is_land: false,
        can_be_commander: false,
      },
    },
    1,
  );
}
