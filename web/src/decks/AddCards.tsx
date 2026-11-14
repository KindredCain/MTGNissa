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
import { api, type DTO } from "../api/client";
import { Modal, validation_error } from "../ui/feedback";
import { LoadMore } from "../ui/controls";
import { context_card } from "./model";
import { get_contexts } from "./contexts";
import type { EditorStore } from "./store";
import { PreviewBubble } from "./Board";
import styles from "./decks.module.css";
const quantity_schema = z.object({
  quantity: z.number().int().min(1).max(4294967295),
});
function candidate_options(
  deck_id: string,
  submitted: { q: string; revision: number } | null,
) {
  return infiniteQueryOptions({
    queryKey: ["deck_candidates", deck_id, submitted?.q, submitted?.revision],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks/{deck_id}/cards/search",
        { q: submitted?.q, cursor: pageParam ?? undefined },
        { path: { deck_id }, signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function AddCards({
  deck_id,
  editor,
  close,
}: {
  deck_id: string;
  editor: EditorStore;
  close: () => void;
}) {
  const client = useQueryClient(),
    search_gate = useRef(false),
    revision = useRef(0);
  const [searching, setSearching] = useState(false);
  const [input, setInput] = useState(""),
    [suggestion, setSuggestion] = useState(""),
    [show, setShow] = useState(false),
    [submitted, setSubmitted] = useState<{
      q: string;
      revision: number;
    } | null>(null),
    [scroll_root, setScrollRoot] = useState<HTMLDivElement | null>(null);
  const list = useInfiniteQuery({
    queryKey: ["deck_candidates", deck_id, submitted?.q, submitted?.revision],
    initialPageParam: null as string | null,
    enabled: submitted !== null,
    placeholderData: keepPreviousData,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks/{deck_id}/cards/search",
        { q: submitted?.q, cursor: pageParam ?? undefined },
        { path: { deck_id }, signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  useEffect(() => {
    const id = setTimeout(() => setSuggestion(input.trim()), 250);
    return () => clearTimeout(id);
  }, [input]);
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
  async function submit(q: string) {
    if (search_gate.current) return;
    search_gate.current = true;
    setSearching(true);
    setShow(false);
    const next = { q: q.trim(), revision: ++revision.current };
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
      close={close}
      close_label="×"
    >
      <form
        className={styles.addTools}
        onSubmit={(event) => {
          event.preventDefault();
          submit(input);
        }}
      >
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
          onBlur={() => setTimeout(() => setShow(false), 150)}
        />
        <button disabled={searching}>搜索</button>
        <span>找到 {list.data?.pages[0].total ?? 0} 张</span>
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
                    submit(item.printed_name);
                  }}
                >
                  {item.printed_name}
                </button>
              ))}
            </div>
          )}
      </form>
      <div className={styles.results} ref={setScrollRoot}>
        {list.data?.pages
          .flatMap((page) => page.items)
          .map((item) => (
            <Candidate
              key={item.card.oracle_id ?? item.card.printing_uuid}
              item={item}
              deck_id={deck_id}
              editor={editor}
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
    </Modal>
  );
}
function Candidate({
  item,
  deck_id,
  editor,
}: {
  item: DTO<"DeckCandidateResponse">;
  deck_id: string;
  editor: EditorStore;
}) {
  const client = useQueryClient(),
    gate = useRef(false),
    [preview, setPreview] = useState<DOMRect | null>(null),
    [show, setShow] = useState(false);
  const form = useForm({
    defaultValues: { quantity: 1 },
    resolver: zodResolver(quantity_schema),
  });
  const add = useMutation({
    mutationFn: async (quantity: number) => {
      const cached = client.getQueryData<DTO<"DeckContextResponse">>([
        "deck_context",
        deck_id,
        "oracle",
        item.card.oracle_id,
      ]);
      const context =
        cached ??
        (await get_contexts(client, deck_id, [item.card.printing_uuid])).get(
          item.card.printing_uuid,
        )!;
      editor.getState().add(context_card(context, quantity), quantity);
    },
    onSettled: () => {
      gate.current = false;
    },
  });
  const context = {
    printing_uuid: item.card.printing_uuid,
    oracle_id: item.card.oracle_id ?? "",
    card: item.card,
    type_group: "other" as const,
    translated_type_line: item.display_type_line,
    translated_oracle_text: item.display_oracle_text,
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
  };
  return (
    <form
      className={styles.resultRow}
      onPointerEnter={(event) => {
        setPreview(event.currentTarget.getBoundingClientRect());
        setShow(true);
      }}
      onPointerLeave={() => setShow(false)}
      onSubmit={(event) => {
        void form.handleSubmit(
          ({ quantity }) => {
            if (gate.current) return;
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
          setPreview(event.currentTarget.getBoundingClientRect());
          setShow(true);
        }}
        onBlur={() => setShow(false)}
        onClick={(event) => {
          setPreview(event.currentTarget.getBoundingClientRect());
          setShow((current) => !current);
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
        disabled={add.isPending}
        {...form.register("quantity", { valueAsNumber: true })}
      />
      <button type="submit" disabled={add.isPending}>
        加入
      </button>
      {show &&
        preview &&
        createPortal(
          <PreviewBubble
            preview={{ card: context_card(context, 1), rect: preview }}
          />,
          document.body,
        )}
    </form>
  );
}
