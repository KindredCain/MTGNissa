import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { api } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardGrid } from "../cards/CardTile";
import { LoadMore, Skeleton, useURLState } from "../ui/controls";
import styles from "../cards/cards.module.css";
import { Icon } from "../ui/Icon";
export function Component() {
  const { params } = useURLState(),
    q = params.get("q") ?? "";
  const list = useInfiniteQuery({
    queryKey: ["search", q],
    enabled: !!q,
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/search",
        { q, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  return (
    <>
      <PageHeader title="全部卡牌" description="查看全部卡牌" tabs />
      <main className={`${pageStyles.content} ${styles.resultsMain}`}>
        <div className={`${pageStyles.breadcrumb} ${styles.resultBreadcrumb}`}>
          <Link to="/sets">卡牌中心</Link>
          <Icon name="chevron-right" size={13} />
          <Link to="/sets">全部卡牌</Link>
          <Icon name="chevron-right" size={13} />
          <span
            title={list.data?.pages[0].executed_query ?? q}
            style={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {list.data?.pages[0].executed_query ?? q}
          </span>
        </div>
        <div className={styles.resultCount}>
          {list.data?.pages[0].total ?? 0} 张卡牌
        </div>
        {list.isPending && !!q ? (
          <Skeleton cards />
        ) : (
          <CardGrid
            cards={list.data?.pages.flatMap((page) => page.items) ?? []}
          />
        )}
        <LoadMore
          loaded_count={
            list.data?.pages.flatMap((page) => page.items).length ?? 0
          }
          end_label="已加载全部搜索结果"
          more_label="向下滚动时自动加载更多搜索结果"
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
