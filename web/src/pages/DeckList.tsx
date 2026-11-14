import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { Link, useNavigate } from "react-router";
import { api } from "../api/client";
import { decks_options } from "../api/lists";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardImage } from "../cards/CardImage";
import { DeckMenu } from "../decks/actions";
import { colors, palette, section_names } from "../decks/model";
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
  const client = useQueryClient();
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
  const create = useMutation({
    mutationFn: () =>
      api.post(
        "/api/v1/decks",
        { name: "新建卡组" },
        { idempotency_key: crypto.randomUUID() },
      ),
    onSuccess: (deck) => {
      void client.invalidateQueries({
        queryKey: ["decks", "list"],
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
                    <Link
                      to={target}
                      aria-label={`查看${deck.name}`}
                      className={styles.cover}
                    >
                      {deck.cover_card ? (
                        <CardImage
                          images={deck.cover_card.images}
                          name={deck.name}
                        />
                      ) : (
                        <div className={styles.emptyCover}>◌</div>
                      )}
                      <time>◷ {relative_time(deck.updated_at)}</time>
                    </Link>
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
                        <Link to={target}>{deck.name}</Link>
                        <DeckMenu deck_id={deck.deck_id} />
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
                        <Link to={target}>查看卡组 →</Link>
                      </div>
                    </div>
                  </article>
                );
              })}
          </div>
        )}
        <LoadMore
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
function relative_time(value: string) {
  const minutes = Math.max(
    0,
    Math.floor((Date.now() - new Date(value).getTime()) / 60000),
  );
  return minutes < 1
    ? "刚刚"
    : minutes < 60
      ? `${minutes} 分钟前`
      : minutes < 1440
        ? `${Math.floor(minutes / 60)} 小时前`
        : minutes < 2880
          ? "昨天"
          : `${Math.floor(minutes / 1440)} 天前`;
}
