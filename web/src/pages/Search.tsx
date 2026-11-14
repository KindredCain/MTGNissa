import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { api } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardGrid } from "../cards/CardTile";
import { LoadMore, Skeleton, Toolbar, useURLState } from "../ui/controls";
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
      <PageHeader title="全部卡牌" description="浏览与搜索万智牌" tabs />
      <main className={pageStyles.content}>
        <div className={pageStyles.breadcrumb}>
          <Link to="/sets">卡牌中心</Link>›<Link to="/sets">全部卡牌</Link>›
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
        <Toolbar count={`${list.data?.pages[0].total ?? 0} 张卡牌`}>
          <span>搜索：{q}</span>
        </Toolbar>
        {list.isPending && !!q ? (
          <Skeleton cards />
        ) : (
          <CardGrid
            cards={list.data?.pages.flatMap((page) => page.items) ?? []}
          />
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
