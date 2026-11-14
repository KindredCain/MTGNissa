import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { api } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import { CardGrid } from "../cards/CardTile";
import { CardImage } from "../cards/CardImage";
import { CardFilters } from "../cards/filters";
import { filter_cards } from "../cards/model";
import {
  LoadMore,
  SearchInput,
  Skeleton,
  Toolbar,
  useURLState,
} from "../ui/controls";
import styles from "../cards/cards.module.css";
export function Component() {
  const { set_code = "" } = useParams(),
    client = useQueryClient();
  const { params, update } = useURLState(async (next, current) => {
    if (next.get("q") === current.get("q")) return;
    const query = {
      set_code,
      language: next.get("language") ?? "",
      q: next.get("q") ?? undefined,
    };
    await client.fetchQuery({
      queryKey: ["cards", "set", query],
      queryFn: ({ signal }) => api.get("/api/v1/cards", query, { signal }),
    });
  });
  return (
    <SetCards
      key={`${set_code}:${params.toString()}`}
      set_code={set_code}
      params={params}
      update={update}
    />
  );
}
function SetCards({
  set_code,
  params,
  update,
}: {
  set_code: string;
  params: URLSearchParams;
  update: ReturnType<typeof useURLState>["update"];
}) {
  const [visible, setVisible] = useState(30);
  const query = {
    set_code,
    language: params.get("language") ?? "",
    q: params.get("q") ?? undefined,
  };
  const list = useQuery({
    queryKey: ["cards", "set", query],
    queryFn: ({ signal }) => api.get("/api/v1/cards", query, { signal }),
  });
  const cards = filter_cards(list.data?.items ?? [], params);
  return (
    <>
      <PageHeader title="全部卡牌" description="浏览万智牌系列" tabs />
      <main className={pageStyles.content}>
        <div className={pageStyles.breadcrumb}>
          <Link to="/sets">卡牌中心</Link>›<Link to="/sets">全部卡牌</Link>›
          <span>
            {list.data?.set.display_name ?? set_code}（
            {list.data?.language.display_name ?? query.language}）
          </span>
        </div>
        <Toolbar count={`${cards.length} 张卡牌`}>
          <SearchInput
            value={query.q ?? ""}
            placeholder="按当前系列卡名搜索"
            onSubmit={(value) => update({ q: value })}
          />
          <CardFilters params={params} update={update} />
        </Toolbar>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          <CardGrid cards={cards.slice(0, visible)} />
        )}
        <LoadMore
          has_more={visible < cards.length}
          loading={false}
          load={() => setVisible((current) => current + 30)}
        />
      </main>
    </>
  );
}
export function TokenComponent() {
  const { set_code = "" } = useParams();
  return <Tokens key={set_code} set_code={set_code} />;
}
function Tokens({ set_code }: { set_code: string }) {
  const [visible, setVisible] = useState(30);
  const list = useQuery({
    queryKey: ["tokens", set_code],
    queryFn: ({ signal }) =>
      api.get("/api/v1/tokens", { set_code }, { signal }),
  });
  return (
    <>
      <PageHeader title="全部卡牌" description="浏览万智牌系列" tabs />
      <main className={pageStyles.content}>
        <div className={pageStyles.breadcrumb}>
          <Link to="/sets">全部卡牌</Link>›
          <span>{list.data?.set.display_name ?? set_code}</span>
        </div>
        <Toolbar count={`${list.data?.total ?? 0} 张卡牌`}>
          <span>Token</span>
        </Toolbar>
        {list.isPending ? (
          <Skeleton cards />
        ) : (
          <div className={styles.grid}>
            {list.data?.items.slice(0, visible).map((item) => (
              <TokenTile key={item.token_uuid} item={item} />
            ))}
          </div>
        )}
        <LoadMore
          has_more={visible < (list.data?.items.length ?? 0)}
          loading={false}
          load={() => setVisible((current) => current + 30)}
        />
      </main>
    </>
  );
}
function TokenTile({
  item,
}: {
  item: NonNullable<
    Awaited<ReturnType<typeof api.get<"/api/v1/tokens">>>
  >["items"][number];
}) {
  const [back, setBack] = useState(false);
  return (
    <article className={styles.tile}>
      <div className={styles.cardWrap}>
        <CardImage images={item.images} name={item.name} back={back} />
        {item.images.has_distinct_back && (
          <button
            className={styles.flip}
            onClick={() => setBack((current) => !current)}
            aria-label={`翻面：${item.name}`}
          >
            ↻
          </button>
        )}
      </div>
    </article>
  );
}
