import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  collection_options,
  wishlist_options,
  personal_params,
  type WishlistItemView,
} from "../api/lists";
import { api, type DTO } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardTile } from "../cards/CardTile";
import { CardFilters } from "../cards/filters";
import { list_sort } from "../cards/model";
import { useCardActions } from "../cards/mutations";
import {
  LoadMore,
  MultiSelect,
  SearchInput,
  Select,
  Skeleton,
  Stepper,
  Toolbar,
  useURLState,
} from "../ui/controls";
import styles from "../cards/cards.module.css";
export function Component() {
  return <Collection />;
}
export function WishlistComponent() {
  return <Wishlist />;
}
function usePersonalFilters(kind: "collection" | "wishlist") {
  const client = useQueryClient();
  const { params, update } = useURLState(async (next) => {
    if (kind === "collection")
      await client.fetchInfiniteQuery(collection_options(next));
    else await client.fetchInfiniteQuery(wishlist_options(next));
  });
  const query = personal_params(params);
  return { params, update, query };
}
function Filters({
  options,
  filters,
}: {
  options: DTO<"CollectionOptionsResponse"> | undefined;
  filters: ReturnType<typeof usePersonalFilters>;
}) {
  const { params, update, query } = filters;
  return (
    <>
      <SearchInput
        value={query.q ?? ""}
        placeholder="按卡牌名称搜索"
        onSubmit={(value) => update({ q: value })}
      />
      <MultiSelect
        label="系列"
        value={query.set_code}
        options={
          options?.sets.map((item) => ({
            value: item.code,
            label: item.display_name,
          })) ?? []
        }
        onChange={(value) => update({ set_code: value })}
      />
      <MultiSelect
        label="语言"
        value={query.language}
        options={
          options?.languages.map((item) => ({
            value: item.code,
            label: item.display_name,
          })) ?? []
        }
        onChange={(value) => update({ language: value })}
      />
      <CardFilters params={params} update={update} sort={false} />
      <Select
        label="排序"
        value={query.sort}
        options={list_sort}
        onChange={(value) =>
          update({
            sort: value,
            merge_sets:
              value === "set"
                ? "false"
                : (params.get("merge_sets") ?? undefined),
          })
        }
      />
    </>
  );
}
function Collection() {
  const filters = usePersonalFilters("collection"),
    { params, update } = filters;
  const query = {
    ...filters.query,
    merge_sets:
      filters.query.sort === "set"
        ? false
        : params.get("merge_sets") === "true",
    merge_languages: params.get("merge_languages") !== "false",
  };
  const options = useQuery({
    queryKey: ["collection", "options"],
    queryFn: ({ signal }) =>
      api.get("/api/v1/collection/options", undefined, { signal }),
  });
  const list = useInfiniteQuery({
    queryKey: ["collection", "list", query],
    refetchOnMount: "always",
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/collection",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const data = list.data?.pages.flatMap((page) => page.items) ?? [],
    summary = list.data?.pages[0].summary;
  const groups = group_items(data);
  return (
    <>
      <PageHeader
        title="我的卡牌"
        description={`共 ${summary?.total_quantity ?? 0} 张牌 · ${summary?.distinct_cards ?? 0} 种卡牌`}
        tabs
      />
      <main className={pageStyles.content}>
        <Toolbar count={`${list.data?.pages[0].total ?? 0} 张卡牌`}>
          <Filters options={options.data} filters={filters} />
        </Toolbar>
        <div className={styles.group}>
          <label>
            <input
              type="checkbox"
              checked={query.merge_sets}
              disabled={query.sort === "set"}
              onChange={(event) =>
                update({ merge_sets: String(event.target.checked) })
              }
            />
            合并系列
          </label>
          <label>
            <input
              type="checkbox"
              checked={query.merge_languages}
              onChange={(event) =>
                update({ merge_languages: String(event.target.checked) })
              }
            />
            合并语言
          </label>
        </div>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          groups.map((group) => (
            <section key={group.key}>
              {group.set && (
                <div className={styles.group}>
                  {group.set.display_name}
                  <small>
                    {group.summary
                      ? `${group.summary.total_quantity} 张 · ${group.summary.distinct_cards} 种`
                      : ""}
                  </small>
                </div>
              )}
              <div className={styles.grid}>
                {group.items.map((item) => (
                  <OwnedTile key={item.card.printing_uuid} item={item} />
                ))}
              </div>
            </section>
          ))
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
function OwnedTile({ item }: { item: DTO<"CollectionItemResponse"> }) {
  const actions = useCardActions(item.card.printing_uuid),
    merged = item.merged_sets || item.merged_languages;
  return (
    <CardTile card={item.card}>
      <div className={styles.actions}>
        {merged ? (
          <div className={styles.readonly}>
            <span>普通 {item.normal_quantity}</span>
            <span>闪卡 {item.foil_quantity}</span>
          </div>
        ) : (
          <>
            <Stepper
              label="普通卡"
              value={item.normal_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "normal", delta })}
            />
            <Stepper
              label="闪卡"
              value={item.foil_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "foil", delta })}
            />
          </>
        )}
      </div>
    </CardTile>
  );
}
function Wishlist() {
  const filters = usePersonalFilters("wishlist");
  const options = useQuery({
    queryKey: ["wishlist", "options"],
    queryFn: ({ signal }) =>
      api.get("/api/v1/wishlist/options", undefined, { signal }),
  });
  const list = useInfiniteQuery({
    queryKey: ["wishlist", "list", filters.query],
    refetchOnMount: "always",
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/wishlist",
        { ...filters.query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const data = list.data?.pages.flatMap((page) => page.items) ?? [];
  return (
    <>
      <PageHeader title="愿望单" description="记录想要收藏的卡牌" tabs />
      <main className={pageStyles.content}>
        <Toolbar count={`${list.data?.pages[0].total ?? 0} 张卡牌`}>
          <Filters options={options.data} filters={filters} />
        </Toolbar>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          group_items(data).map((group) => (
            <section key={group.key}>
              {group.set && (
                <div className={styles.group}>{group.set.display_name}</div>
              )}
              <div className={styles.grid}>
                {group.items.map((item) => (
                  <WishlistTile key={item.card.printing_uuid} item={item} />
                ))}
              </div>
            </section>
          ))
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
function WishlistTile({ item }: { item: WishlistItemView }) {
  const actions = useCardActions(item.card.printing_uuid);
  return (
    <CardTile card={item.card}>
      <div className={styles.actions}>
        <button
          className={styles.star}
          disabled={actions.pending}
          onClick={() => actions.act({ included: !item.included })}
        >
          {item.included ? "★ 移出愿望单" : "☆ 恢复愿望单"}
        </button>
      </div>
    </CardTile>
  );
}
function group_items<
  T extends {
    group_set: DTO<"CardSetResponse"> | null;
    group_summary?: DTO<"OwnershipTotalsResponse"> | null;
  },
>(items: T[]) {
  const groups: {
    key: string;
    set: T["group_set"];
    summary: T["group_summary"];
    items: T[];
  }[] = [];
  for (const item of items) {
    const key = item.group_set?.code ?? "all";
    let group = groups.at(-1);
    if (!group || group.key !== key) {
      group = {
        key,
        set: item.group_set,
        summary: item.group_summary,
        items: [],
      };
      groups.push(group);
    }
    group.items.push(item);
  }
  return groups;
}
