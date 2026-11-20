import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { sets_options } from "../api/lists";
import { Link, useNavigate } from "react-router";
import { api } from "../api/client";
import { language_name, set_sort_names } from "../shared/display-names";
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
      <PageHeader title="全部卡牌" description="查看全部卡牌" tabs />
      <main className={pageStyles.content}>
        <div className={pageStyles.breadcrumb}>
          <Link to="/sets">卡牌中心</Link>
          <span>›</span>
          <span>全部卡牌</span>
        </div>
        <Toolbar
          count={`已加载 ${items.filter((item) => item.depth === 0).length} 个父系列 · ${items.length} 行`}
        >
          <SearchInput
            value={query.q ?? ""}
            placeholder="按系列名称或缩写搜索"
            onSubmit={(value) => update({ q: value })}
          />
          <MultiSelect
            label="语言"
            all_label="全部语言"
            value={query.language}
            options={
              options.data?.languages.map((item) => ({
                value: item.code,
                label: language_name(item.code),
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
            prefix="排序："
            value={query.sort}
            options={sorts.map((value) => ({
              value,
              label: set_sort_names[value],
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
                <th>卡牌</th>
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
                        <Link
                          to={target}
                          onClick={(event) => event.stopPropagation()}
                        >
                          {item.display_name}
                        </Link>
                      ) : (
                        item.display_name
                      )}
                      <small>{item.code}</small>
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
                            {language_name(language.code)}
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
          more_label="向下滚动时自动加载更多系列"
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
