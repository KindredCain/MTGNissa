import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { Link, useLocation, useNavigate } from "react-router";
import { useEffect, useState } from "react";
import { api, create_idempotent_post } from "../api/client";
import { decks_options } from "../api/lists";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardImage } from "../cards/CardImage";
import { DeckMenu } from "../decks/actions";
import { colors, palette, section_names } from "../decks/model";
import { relative_time } from "../decks/time";
import {
  LoadMore,
  SearchInput,
  Select,
  Skeleton,
  Toolbar,
  useURLState,
} from "../ui/controls";
import styles from "../decks/decks.module.css";
export function Component() {
  const [post_intent] = useState(create_idempotent_post);
  const client = useQueryClient(),
    location = useLocation();
  const { params, update } = useURLState((next) =>
      client.fetchInfiniteQuery(decks_options(next)),
    ),
    navigate = useNavigate();
  const query = {
    q: params.get("q") ?? undefined,
    sort: (params.get("sort") ?? "updated_desc") as
      "updated_desc" | "created_desc" | "name_asc" | "card_count_desc",
  };
  const list = useInfiniteQuery({
    queryKey: ["decks", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  useEffect(() => {
    // A navigation to this same route can leave the existing list mounted.
    void client.refetchQueries(
      { queryKey: ["decks", "list"], type: "active", stale: true },
      { cancelRefetch: false },
    );
  }, [client, location.key]);
  const create = useMutation({
    mutationFn: () => post_intent("/api/v1/decks", { name: "新建卡组" }),
    onSuccess: (deck) => {
      void client.invalidateQueries({
        queryKey: ["decks", "list"],
        refetchType: "none",
      });
      void client.invalidateQueries({
        queryKey: ["operation_logs"],
        refetchType: "none",
      });
      navigate(`/decks/${encodeURIComponent(deck.deck_id)}/edit`);
    },
  });
  return (
    <>
      <PageHeader title="卡组" description="管理、查看和继续编辑我的卡组">
        <button
          className={pageStyles.primary}
          disabled={create.isPending}
          onClick={() => create.mutate()}
        >
          ＋ 新建卡组
        </button>
      </PageHeader>
      <main className={pageStyles.content}>
        <Toolbar count={`共 ${list.data?.pages[0].total ?? 0} 个卡组`}>
          <SearchInput
            placeholder="按卡组名称搜索"
            value={query.q ?? ""}
            onSubmit={(value) => update({ q: value })}
          />
          <Select
            label="卡组排序"
            value={query.sort}
            options={[
              { value: "updated_desc", label: "最近更新" },
              { value: "created_desc", label: "创建时间" },
              { value: "name_asc", label: "名称" },
              { value: "card_count_desc", label: "卡牌总数" },
            ]}
            onChange={(value) => update({ sort: value })}
          />
        </Toolbar>
        {list.isPending ? (
          <Skeleton />
        ) : (
          <div className={styles.deckGrid}>
            {list.data?.pages
              .flatMap((page) => page.items)
              .map((deck) => {
                const total = Object.values(deck.color_distribution).reduce(
                    (sum, value) => sum + value,
                    0,
                  ),
                  target = `/decks/${encodeURIComponent(deck.deck_id)}`;
                return (
                  <article className={styles.deckCard} key={deck.deck_id}>
                    <div className={styles.cover}>
                      {deck.cover_card ? (
                        <CardImage
                          images={deck.cover_card.images}
                          name={deck.name}
                        />
                      ) : (
                        <div className={styles.emptyCover}>◌</div>
                      )}
                      <time>◷ {relative_time(deck.updated_at)}</time>
                    </div>
                    <div className={styles.colorBand}>
                      {total > 0 &&
                        colors.map(
                          (color, index) =>
                            deck.color_distribution[color] > 0 && (
                              <span
                                key={color}
                                style={{
                                  width: `${(deck.color_distribution[color] / total) * 100}%`,
                                  background: palette[index],
                                }}
                              />
                            ),
                        )}
                    </div>
                    <div className={styles.deckInfo}>
                      <div className={styles.deckName}>
                        <Link
                          to={target}
                          aria-label={`查看${deck.name}`}
                          className={`${styles.deckTitle} ${styles.deckLink}`}
                        >
                          {deck.name}
                        </Link>
                        <div className={styles.deckMenu}>
                          <DeckMenu deck_id={deck.deck_id} />
                        </div>
                      </div>
                      <div className={styles.deckCounts}>
                        <span>
                          {Object.entries(deck.section_counts)
                            .filter(([, count]) => count > 0)
                            .map(
                              ([type, count]) =>
                                `${section_names[type as keyof typeof section_names]} ${count}`,
                            )
                            .join(" · ")}
                        </span>
                        <span>共 {deck.counted_total} 张</span>
                      </div>
                      <div className={styles.deckFooter}>
                        {deck.exception && (
                          <span
                            className={
                              deck.exception.type === "shortage"
                                ? styles.shortage
                                : styles.usage
                            }
                          >
                            {deck.exception.type === "shortage"
                              ? "⚠ 数量不足"
                              : "▱ 卡组占用"}
                          </span>
                        )}
                        <span className={styles.deckDetailLink}>
                          查看卡组 →
                        </span>
                      </div>
                    </div>
                  </article>
                );
              })}
          </div>
        )}
        <LoadMore
          loaded_count={
            list.data?.pages.flatMap((page) => page.items).length ?? 0
          }
          end_label="已加载全部卡组"
          failed={list.isFetchNextPageError}
          has_more={!!list.hasNextPage}
          loading={list.isFetchingNextPage}
          load={() => {
            void list.fetchNextPage();
          }}
        />
      </main>
    </>
  );
}
