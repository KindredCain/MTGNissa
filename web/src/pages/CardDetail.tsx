import { useLayoutEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import { api } from "../api/client";
import {
  finish_names,
  language_name,
  rarity_names,
} from "../shared/display-names";
import { pageStyles } from "../app/Layout";
import { CardImage } from "../cards/CardImage";
import { useCardActions } from "../cards/mutations";
import { Icon } from "../ui/Icon";
import { Mana } from "../cards/Mana";
import { Select, Skeleton, Stepper } from "../ui/controls";
import styles from "../cards/detail.module.css";
export function Component() {
  const { printing_uuid = "" } = useParams();
  return <Detail key={printing_uuid} printing_uuid={printing_uuid} />;
}
function Detail({ printing_uuid }: { printing_uuid: string }) {
  const [back, setBack] = useState(false),
    [tab, setTab] = useState("text");
  const navigate = useNavigate(),
    client = useQueryClient(),
    location = useLocation(),
    active_location = useRef<typeof location | null>(location),
    revision = useRef(0),
    actions = useCardActions(printing_uuid);
  useLayoutEffect(() => {
    active_location.current = location;
    return () => {
      active_location.current = null;
    };
  }, [location]);
  async function switch_printing(next_uuid: string) {
    const intent = ++revision.current;
    if (next_uuid === printing_uuid) return;
    try {
      await client.fetchQuery({
        queryKey: ["card", next_uuid],
        queryFn: ({ signal }) =>
          api.get("/api/v1/cards/{printing_uuid}", undefined, {
            path: { printing_uuid: next_uuid },
            signal,
          }),
      });
      if (intent === revision.current && active_location.current === location)
        navigate(`/cards/${encodeURIComponent(next_uuid)}`, {
          state: { scroll: "preserve" },
        });
    } catch {
      /* QueryCache reports the failure; keep the current printing visible. */
    }
  }
  const query = useQuery({
    queryKey: ["card", printing_uuid],
    queryFn: ({ signal }) =>
      api.get("/api/v1/cards/{printing_uuid}", undefined, {
        path: { printing_uuid },
        signal,
      }),
  });
  const data = query.data;
  if (!data)
    return (
      <main className={pageStyles.content}>
        {query.isPending && <Skeleton />}
      </main>
    );
  const { card } = data,
    face = data.faces[back ? 1 : 0] ?? data.faces[0],
    text_faces = card.images.has_distinct_back
      ? face
        ? [face]
        : []
      : data.faces;
  const demand = data.deck_usage.decks.reduce(
      (sum, deck) => sum + deck.quantity,
      0,
    ),
    shortage = Math.max(0, demand - data.logical_collection.total_quantity);
  return (
    <main className={pageStyles.content}>
      <div className={pageStyles.breadcrumb}>
        <Link
          to={`/sets/${encodeURIComponent(card.set.code)}/cards?language=${encodeURIComponent(card.language.code)}`}
        >
          {card.set.display_name}（{language_name(card.language.code)}）
        </Link>
        <Icon name="chevron-right" size={13} />
        {card.display_name}
      </div>
      <div className={styles.layout}>
        <div>
          <CardImage
            name={card.display_name}
            images={card.images}
            back={back}
          />
          {card.images.has_distinct_back && (
            <button
              className={styles.faceButton}
              onClick={() => setBack((value) => !value)}
            >
              <Icon name="refresh-cw" size={15} />{" "}
              {back ? "查看正面" : "查看反面"}
            </button>
          )}
        </div>
        <section>
          <div className={styles.title}>
            <h1>{face?.display_name ?? card.display_name}</h1>
            <Mana cost={face?.mana_cost ?? null} />
          </div>
          <p className={styles.english}>{face?.name ?? card.name}</p>
          <div className={styles.metadata}>
            <span>
              <small>系列</small>
              {card.set.display_name} · {card.set.code}
            </span>
            <span>
              <small>收藏编号</small>#{card.collector_number}
            </span>
            <span>
              <small>稀有度</small>
              {rarity_names[card.rarity]}
            </span>
          </div>
          <div className={styles.tabs}>
            <button
              aria-pressed={tab === "text"}
              onClick={() => setTab("text")}
            >
              卡牌详情
            </button>
            <button
              aria-pressed={tab === "rulings"}
              onClick={() => setTab("rulings")}
            >
              官方释疑
            </button>
            <button
              aria-pressed={tab === "tags"}
              onClick={() => setTab("tags")}
            >
              功能标签
            </button>
          </div>
          {tab === "text" && (
            <div className={styles.rules}>
              {text_faces.map((text_face, index) => (
                <article key={text_face.face_index} className={styles.ruleFace}>
                  {index > 0 && (
                    <>
                      <div className={styles.title}>
                        <h2>{text_face.display_name}</h2>
                        <Mana cost={text_face.mana_cost} />
                      </div>
                      <p className={styles.english}>{text_face.name}</p>
                    </>
                  )}
                  <h3>类别</h3>
                  <p>{text_face.display_type_line}</p>
                  <h3>规则文字</h3>
                  <p>{text_face.display_oracle_text}</p>
                  {text_face.display_flavor_text && (
                    <blockquote>{text_face.display_flavor_text}</blockquote>
                  )}
                  {text_face.power !== null && (
                    <p className={styles.power}>
                      {text_face.power} / {text_face.toughness}
                    </p>
                  )}
                  {text_face.loyalty && <p>忠诚值：{text_face.loyalty}</p>}
                </article>
              ))}
            </div>
          )}
          {tab === "rulings" && (
            <div className={styles.rulings}>
              <div className={styles.contentHeader}>
                <h2>官方释疑</h2>
                <span>{data.rulings.length} 条</span>
              </div>
              {data.rulings.map((ruling, index) => (
                <article key={index}>
                  <time>{ruling.published_at}</time>
                  <p>{ruling.text}</p>
                </article>
              ))}
            </div>
          )}
          {tab === "tags" && (
            <div>
              <div className={styles.contentHeader}>
                <h2>功能标签</h2>
                <span>{data.oracle_tags.length} 个标签</span>
              </div>
              <div className={styles.tags}>
                {data.oracle_tags.map((tag) => (
                  <Link
                    key={tag.tag_id}
                    to={`/search?q=${encodeURIComponent(tag.query)}`}
                  >
                    {tag.query}
                    <span>
                      <Icon name="arrow-up-right" size={13} />
                    </span>
                  </Link>
                ))}
              </div>
              <p className={styles.tagNote}>
                标签来自 Scryfall Tagger
                社区数据；点击具体标签进入对应语法搜索结果。
              </p>
            </div>
          )}
        </section>
        <aside className={styles.aside}>
          <section className={styles.collectionPanel}>
            <div className={styles.asideTitle}>
              <Link to="/collection">
                我的收藏 <Icon name="arrow-up-right" size={13} />
              </Link>
              <button
                disabled={actions.pending}
                aria-pressed={data.wishlist.included}
                onClick={() =>
                  actions.act({ included: !data.wishlist.included })
                }
              >
                <Icon name="star" filled={data.wishlist.included} /> 愿望单
              </button>
            </div>
            <Stepper
              label={finish_names.normal}
              value={data.collection.normal_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "normal", delta })}
            />
            <Stepper
              label={finish_names.foil}
              value={data.collection.foil_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "foil", delta })}
            />
            <div className={styles.versionTotal}>
              <span>当前版本拥有</span>
              <strong
                title={`${finish_names.normal} ${data.collection.normal_quantity}，${finish_names.foil} ${data.collection.foil_quantity}`}
              >
                {data.collection.normal_quantity}:
                {data.collection.foil_quantity}*
              </strong>
            </div>
            <div className={styles.total}>
              该卡收藏数量{" "}
              <strong>共 {data.logical_collection.total_quantity} 张</strong>
              <small>
                {finish_names.normal} {data.logical_collection.normal_quantity}{" "}
                · {finish_names.foil} {data.logical_collection.foil_quantity}
              </small>
            </div>
          </section>
          <section className={styles.languagePanel}>
            <h3>当前系列语言</h3>
            <p className={styles.printing}>
              {card.set.display_name} · {card.set.code} · #
              {card.collector_number}
            </p>
            <div className={styles.languageSelect}>
              <Select
                label="当前系列语言"
                icon="languages"
                value={card.printing_uuid}
                options={data.languages.map((language) => ({
                  value: language.printing_uuid,
                  label: language_name(language.language.code),
                }))}
                onChange={(value) => void switch_printing(value)}
              />
              <small>{data.languages.length} 种可选</small>
            </div>
          </section>
          <section className={styles.printingPanel}>
            <h3>
              其他系列版本 <small>共 {data.other_versions.length} 个版本</small>
            </h3>
            <div className={styles.versions}>
              {data.other_versions.map((version) => (
                <article
                  key={`${version.set.code}:${version.collector_number}`}
                >
                  <div className={styles.versionHeader}>
                    <span className={styles.versionIdentity}>
                      <span title={version.set.display_name}>
                        {version.set.display_name}
                      </span>
                      <span>
                        · {version.set.code} · #{version.collector_number}
                      </span>
                    </span>
                    <small>
                      拥有 {version.normal_quantity + version.foil_quantity}
                    </small>
                  </div>
                  <div className={styles.languages}>
                    {version.languages.map((language) => (
                      <Link
                        key={language.printing_uuid}
                        to={`/cards/${encodeURIComponent(language.printing_uuid)}`}
                        onClick={(event) => {
                          if (
                            event.button !== 0 ||
                            event.ctrlKey ||
                            event.metaKey ||
                            event.shiftKey ||
                            event.altKey
                          )
                            return;
                          event.preventDefault();
                          void switch_printing(language.printing_uuid);
                        }}
                      >
                        {language_name(language.language.code)}
                      </Link>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </section>
          <section className={styles.deckUsage}>
            <h3>卡组使用情况</h3>
            <p className={styles.usageSummary}>
              出现在 {data.deck_usage.total_decks} 个卡组 · 总需求 {demand} ·
              跨卡组缺口 {shortage}
            </p>
            {data.deck_usage.decks.map((deck) => (
              <Link
                key={deck.deck_id}
                to={`/decks/${encodeURIComponent(deck.deck_id)}`}
              >
                <span>
                  {deck.name} <Icon name="arrow-up-right" size={13} />
                </span>
                <span>{deck.quantity}</span>
              </Link>
            ))}
          </section>
        </aside>
      </div>
    </main>
  );
}
