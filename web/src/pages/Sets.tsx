import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { sets_options } from "../api/lists";
import { Link, useNavigate } from "react-router";
import { api } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import {
  LoadMore,
  MultiSelect,
  SearchInput,
  Select,
  SortDivider,
  Skeleton,
  Toolbar,
  useURLState,
} from "../ui/controls";
import styles from "./sets.module.css";
const sorts = [
  "released_desc",
  "released_asc",
  "name_asc",
  "name_desc",
  "code_asc",
  "code_desc",
  "card_count_desc",
  "card_count_asc",
] as const;
export function Component() {
  const client = useQueryClient();
  const { params, update } = useURLState((next) =>
      client.fetchInfiniteQuery(sets_options(next)),
    ),
    navigate = useNavigate();
  const query = {
    q: params.get("q") ?? undefined,
    language: params.getAll("language"),
    set_type: params.getAll("set_type"),
    sort: (params.get("sort") ?? "released_desc") as (typeof sorts)[number],
  };
  const options = useQuery({
    queryKey: ["sets", "options"],
    queryFn: ({ signal }) =>
      api.get("/api/v1/sets/options", undefined, { signal }),
  });
  const list = useInfiniteQuery({
    queryKey: ["sets", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/sets",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const items = list.data?.pages.flatMap((page) => page.items) ?? [];
  return (
    <>
      <PageHeader title="全部卡牌" description="浏览万智牌系列" tabs />
      <main className={pageStyles.content}>
        <div className={pageStyles.breadcrumb}>
          <Link to="/sets">卡牌中心</Link>
          <span>›</span>
          <span>全部卡牌</span>
        </div>
        <Toolbar count={`共 ${list.data?.pages[0].total ?? 0} 个系列`}>
          <SearchInput
            value={query.q ?? ""}
            placeholder="按系列名称或缩写搜索"
            onSubmit={(value) => update({ q: value })}
          />
          <MultiSelect
            label="语言"
            value={query.language}
            options={
              options.data?.languages.map((item) => ({
                value: item.code,
                label: item.display_name,
              })) ?? []
            }
            onChange={(value) => update({ language: value })}
          />
          <MultiSelect
            label="系列类型"
            value={query.set_type}
            options={
              options.data?.set_types.map((value) => ({
                value,
                label: value,
              })) ?? []
            }
            onChange={(value) => update({ set_type: value })}
          />
          <SortDivider />
          <Select
            label="系列排序"
            value={query.sort}
            options={sorts.map((value, index) => ({
              value,
              label: [
                "发行时间：新到旧",
                "发行时间：旧到新",
                "名称：A–Z",
                "名称：Z–A",
                "缩写：A–Z",
                "缩写：Z–A",
                "牌数：多到少",
                "牌数：少到多",
              ][index],
            }))}
            onChange={(value) => update({ sort: value })}
          />
        </Toolbar>
        {list.isPending ? (
          <Skeleton />
        ) : (
          <table className={styles.table}>
            <thead>
              <tr>
                <th>系列</th>
                <th>类型</th>
                <th>牌数</th>
                <th>发行日期</th>
                <th>可用语言</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => {
                const target =
                  item.set_type === "token"
                    ? `/sets/${encodeURIComponent(item.code)}/tokens`
                    : item.default_language
                      ? `/sets/${encodeURIComponent(item.code)}/cards?language=${encodeURIComponent(item.default_language)}`
                      : null;
                return (
                  <tr
                    key={item.code}
                    className={target ? styles.clickable : ""}
                    onClick={() => target && navigate(target)}
                  >
                    <td style={{ paddingLeft: 18 + item.depth * 22 }}>
                      {target ? (
                        <Link to={target}>{item.display_name}</Link>
                      ) : (
                        item.display_name
                      )}
                      <small>
                        {item.name} · {item.code}
                      </small>
                    </td>
                    <td>
                      <span className={styles.code}>{item.set_type}</span>
                    </td>
                    <td>{item.card_count}</td>
                    <td>{item.released_at ?? "—"}</td>
                    <td>
                      <div className={styles.languages}>
                        {item.languages.map((language) => (
                          <button
                            key={language.code}
                            aria-disabled={!language.available}
                            className={
                              !language.available ? styles.unavailable : ""
                            }
                            onClick={(event) => {
                              event.stopPropagation();
                              if (language.available)
                                navigate(
                                  `/sets/${encodeURIComponent(item.code)}/cards?language=${encodeURIComponent(language.code)}`,
                                );
                            }}
                          >
                            {language.display_name}
                          </button>
                        ))}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
        <LoadMore
          loaded_count={items.length}
          end_label="已加载全部系列"
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
