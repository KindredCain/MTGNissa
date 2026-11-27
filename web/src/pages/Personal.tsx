import { useId, useState, type ReactNode } from "react";
import { Link } from "react-router";
import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  collection_options,
  wishlist_options,
  collection_params,
  wishlist_params,
  type WishlistItemView,
} from "../api/lists";
import { api, type DTO } from "../api/client";
import {
  finish_names,
  language_name,
  personal_sort_names,
} from "../shared/display-names";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardTile } from "../cards/CardTile";
import { CardFilters } from "../cards/filters";
import { collection_sorts, wishlist_sorts } from "../cards/model";
import { useCardActions } from "../cards/mutations";
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
import styles from "../cards/cards.module.css";
import { Icon } from "../ui/Icon";
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
  const query =
    kind === "collection" ? collection_params(params) : wishlist_params(params);
  return { kind, params, update, query };
}
function Filters({
  options,
  filters,
}: {
  options: DTO<"CollectionOptionsResponse"> | undefined;
  filters: ReturnType<typeof usePersonalFilters>;
}) {
  const { kind, params, update, query } = filters;
  const sort_options =
    kind === "collection"
      ? collection_sorts.map((value) => ({
          value,
          label: personal_sort_names.collection[value],
        }))
      : wishlist_sorts.map((value) => ({
          value,
          label: personal_sort_names.wishlist[value],
        }));
  return (
    <>
      <SearchInput
        value={query.q ?? ""}
        placeholder={
          kind === "collection" ? "按卡名搜索我的卡牌" : "按卡名搜索愿望单"
        }
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
            label: language_name(item.code),
          })) ?? []
        }
        onChange={(value) => update({ language: value })}
      />
      <CardFilters params={params} update={update} sort={false} />
      <SortDivider />
      <Select
        label="排序"
        prefix="排序："
        value={query.sort}
        options={sort_options}
        onChange={(value) =>
          update(
            kind === "collection"
              ? {
                  sort: value,
                  merge_sets:
                    value === "set_desc"
                      ? "false"
                      : (params.get("merge_sets") ?? undefined),
                }
              : { sort: value },
          )
        }
      />
    </>
  );
}
function Collection() {
  const filters = usePersonalFilters("collection"),
    { params, update } = filters;
  const query = collection_params(params);
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
      <PageHeader title="我的卡牌" description="管理拥有的卡牌收藏" tabs />
      <main className={pageStyles.content} data-card-grid-page>
        <nav
          className={pageStyles.breadcrumb}
          aria-label="面包屑"
          data-preview-header
        >
          <Link to="/sets">卡牌中心</Link>
          <Icon name="chevron-right" size={13} />
          <Link to="/collection" aria-current="page">
            我的卡牌
          </Link>
        </nav>
        <Toolbar
          count={
            <span className={styles.mergeAndCount}>
              <label className={styles.mergeToggle}>
                合并不同系列
                <input
                  type="checkbox"
                  role="switch"
                  checked={query.merge_sets}
                  disabled={query.sort === "set_desc"}
                  onChange={(event) =>
                    update({ merge_sets: String(event.target.checked) })
                  }
                />
              </label>
              <label className={styles.mergeToggle}>
                合并不同语言
                <input
                  type="checkbox"
                  role="switch"
                  checked={query.merge_languages}
                  onChange={(event) =>
                    update({ merge_languages: String(event.target.checked) })
                  }
                />
              </label>
              <span>
                共 {(summary?.total_quantity ?? 0).toLocaleString("zh-CN")} 张，
                {(summary?.distinct_cards ?? 0).toLocaleString("zh-CN")} 种
              </span>
            </span>
          }
        >
          <Filters options={options.data} filters={filters} />
        </Toolbar>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          groups.map((group) => (
            <PersonalGroup
              key={group.key}
              set={group.set}
              summary={group.summary}
            >
              <div className={styles.grid}>
                {group.items.map((item) => (
                  <OwnedTile key={item.card.printing_uuid} item={item} />
                ))}
              </div>
            </PersonalGroup>
          ))
        )}
        <LoadMore
          loaded_count={data.length}
          end_label="已加载全部卡牌"
          more_label="向下滚动时显示更多收藏卡牌"
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
      <div className={styles.quantityBar}>
        {(["normal", "foil"] as const).map((finish) => {
          const value =
              finish === "normal" ? item.normal_quantity : item.foil_quantity,
            label = finish_names[finish];
          return (
            <div key={finish} className={styles.quantityStep}>
              {!merged && (
                <button
                  type="button"
                  aria-label={`减少${label}`}
                  disabled={actions.pending || value === 0}
                  onClick={() => actions.act({ finish, delta: -1 })}
                >
                  <Icon name="minus" size={8} />
                </button>
              )}
              <span title={`${label} ${value}`}>
                {label} <output>{value}</output>
              </span>
              {!merged && (
                <button
                  type="button"
                  aria-label={`增加${label}`}
                  disabled={actions.pending}
                  onClick={() => actions.act({ finish, delta: 1 })}
                >
                  <Icon name="plus" size={8} />
                </button>
              )}
            </div>
          );
        })}
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
      <PageHeader title="愿望单" description="管理我的愿望单" tabs />
      <main className={pageStyles.content} data-card-grid-page>
        <nav
          className={pageStyles.breadcrumb}
          aria-label="面包屑"
          data-preview-header
        >
          <Link to="/sets">卡牌中心</Link>
          <Icon name="chevron-right" size={13} />
          <Link to="/wishlist" aria-current="page">
            愿望单
          </Link>
        </nav>
        <Toolbar>
          <Filters options={options.data} filters={filters} />
        </Toolbar>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          group_items(data).map((group) => (
            <PersonalGroup key={group.key} set={group.set}>
              <div className={styles.grid}>
                {group.items.map((item) => (
                  <WishlistTile key={item.card.printing_uuid} item={item} />
                ))}
              </div>
            </PersonalGroup>
          ))
        )}
        <LoadMore
          loaded_count={data.length}
          end_label="已加载全部愿望单卡牌"
          more_label="向下滚动时显示更多愿望单卡牌"
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
function PersonalGroup({
  set,
  summary,
  children,
}: {
  set: DTO<"CardSetResponse"> | null;
  summary?: DTO<"OwnershipTotalsResponse"> | null;
  children: ReactNode;
}) {
  const [expanded, setExpanded] = useState(true),
    content_id = useId();
  return (
    <section className={styles.personalGroup}>
      {set && (
        <button
          type="button"
          className={styles.groupToggle}
          aria-expanded={expanded}
          aria-controls={content_id}
          onClick={() => setExpanded((value) => !value)}
        >
          <span className={styles.groupIdentity}>
            <small>{set.code}</small>
            <span>{set.display_name}</span>
            {set.released_at && <small>{set.released_at}</small>}
          </span>
          <span className={styles.groupDivider} aria-hidden="true" />
          {summary && (
            <small>
              共 {summary.total_quantity} 张，{summary.distinct_cards} 种
            </small>
          )}
          <Icon name={expanded ? "chevron-up" : "chevron-down"} size={13} />
        </button>
      )}
      <div id={content_id} hidden={!!set && !expanded}>
        {children}
      </div>
    </section>
  );
}
function WishlistTile({ item }: { item: WishlistItemView }) {
  const actions = useCardActions(item.card.printing_uuid);
  return (
    <CardTile card={item.card}>
      <div className={styles.wishlistFooter}>
        <span title={item.card.display_name}>{item.card.display_name}</span>
        <button
          type="button"
          className={styles.star}
          aria-label={item.included ? "移出愿望单" : "恢复愿望单"}
          aria-pressed={item.included}
          disabled={actions.pending}
          onClick={(event) => {
            event.stopPropagation();
            actions.act({ included: !item.included });
          }}
        >
          <Icon name="star" filled={item.included} />
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
