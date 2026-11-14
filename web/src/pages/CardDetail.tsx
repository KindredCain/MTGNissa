import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router";
import { api } from "../api/client";
import { pageStyles } from "../app/Layout";
import { CardImage } from "../cards/CardImage";
import { useCardActions } from "../cards/mutations";
import { Mana } from "../cards/Mana";
import { Select, Skeleton, Stepper } from "../ui/controls";
import { rarity_options } from "../cards/model";
import styles from "../cards/detail.module.css";
export function Component() {
  const { printing_uuid = "" } = useParams();
  return <Detail key={printing_uuid} printing_uuid={printing_uuid} />;
}
function Detail({ printing_uuid }: { printing_uuid: string }) {
  const [back, setBack] = useState(false),
    [tab, setTab] = useState("text");
  const navigate = useNavigate(),
    actions = useCardActions(printing_uuid);
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
    face = data.faces[back ? 1 : 0] ?? data.faces[0];
  return (
    <main className={pageStyles.content}>
      <div className={pageStyles.breadcrumb}>
        <Link
          to={`/sets/${encodeURIComponent(card.set.code)}/cards?language=${encodeURIComponent(card.language.code)}`}
        >
          {card.set.display_name}（{card.language.display_name}）
        </Link>
        ›{card.display_name}
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
              {card.set.display_name}
              <small>
                {card.set.code} · #{card.collector_number}
              </small>
            </span>
            <span>
              {
                rarity_options.find((option) => option.value === card.rarity)
                  ?.label
              }
              <small>稀有度</small>
            </span>
            <span>
              {card.cmc}
              <small>法术力</small>
            </span>
          </div>
          <div className={styles.tabs}>
            <button
              aria-pressed={tab === "text"}
              onClick={() => setTab("text")}
            >
              卡牌信息
            </button>
            <button
              aria-pressed={tab === "rulings"}
              onClick={() => setTab("rulings")}
            >
              官方释疑 {data.rulings.length}
            </button>
            <button
              aria-pressed={tab === "tags"}
              onClick={() => setTab("tags")}
            >
              功能标签 {data.oracle_tags.length}
            </button>
          </div>
          {tab === "text" && (
            <div className={styles.rules}>
              <h3>{face?.display_type_line}</h3>
              <p>{face?.display_oracle_text}</p>
              {face?.display_flavor_text && (
                <blockquote>{face.display_flavor_text}</blockquote>
              )}
              {face?.power !== null && face?.power !== undefined && (
                <p className={styles.power}>
                  {face.power} / {face.toughness}
                </p>
              )}
              {face?.loyalty && <p>忠诚值：{face.loyalty}</p>}
            </div>
          )}
          {tab === "rulings" && (
            <div className={styles.rulings}>
              {data.rulings.map((ruling, index) => (
                <article key={index}>
                  <time>{ruling.published_at}</time>
                  <p>{ruling.text}</p>
                </article>
              ))}
            </div>
          )}
          {tab === "tags" && (
            <div className={styles.tags}>
              {data.oracle_tags.map((tag) => (
                <Link
                  key={tag.tag_id}
                  to={`/search?q=${encodeURIComponent(tag.query)}`}
                >
                  {tag.query}
                  <span>↗</span>
                </Link>
              ))}
            </div>
          )}
        </section>
        <aside className={styles.aside}>
          <section>
            <div className={styles.asideTitle}>
              <Link to="/collection">我的收藏 ↗</Link>
              <button
                disabled={actions.pending}
                onClick={() =>
                  actions.act({ included: !data.wishlist.included })
                }
              >
                {data.wishlist.included ? "★ 已收藏" : "☆ 愿望单"}
              </button>
            </div>
            <Stepper
              label="普通卡"
              value={data.collection.normal_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "normal", delta })}
            />
            <Stepper
              label="闪卡"
              value={data.collection.foil_quantity}
              disabled={actions.pending}
              onChange={(delta) => actions.act({ finish: "foil", delta })}
            />
            <div className={styles.total}>
              该卡收藏数量{" "}
              <strong>{data.logical_collection.total_quantity}</strong>
              <small>
                普通 {data.logical_collection.normal_quantity} · 闪卡{" "}
                {data.logical_collection.foil_quantity}
              </small>
            </div>
          </section>
          <section>
            <h3>
              当前系列语言 <small>{data.languages.length}</small>
            </h3>
            <Select
              label="当前系列语言"
              value={card.printing_uuid}
              options={data.languages.map((language) => ({
                value: language.printing_uuid,
                label: language.language.display_name,
              }))}
              onChange={(value) =>
                navigate(`/cards/${encodeURIComponent(value)}`)
              }
            />
          </section>
          <section>
            <h3>
              其他系列版本 <small>{data.other_versions.length}</small>
            </h3>
            <div className={styles.versions}>
              {data.other_versions.map((version) => (
                <article
                  key={`${version.set.code}:${version.collector_number}`}
                >
                  <div>
                    {version.set.display_name} · {version.set.code} · #
                    {version.collector_number}
                    <small>
                      拥有 {version.normal_quantity + version.foil_quantity}
                    </small>
                  </div>
                  <div className={styles.languages}>
                    {version.languages.map((language) => (
                      <Link
                        key={language.printing_uuid}
                        to={`/cards/${encodeURIComponent(language.printing_uuid)}`}
                      >
                        {language.language.display_name}{" "}
                        {language.normal_quantity + language.foil_quantity ||
                          ""}
                      </Link>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </section>
        </aside>
      </div>
      <section className={styles.deckUsage}>
        <h2>
          卡组使用情况 <small>{data.deck_usage.total_decks}</small>
        </h2>
        {data.deck_usage.decks.map((deck) => (
          <Link
            key={deck.deck_id}
            to={`/decks/${encodeURIComponent(deck.deck_id)}`}
          >
            <span>{deck.name}</span>
            <span>{deck.quantity} 张</span>
          </Link>
        ))}
      </section>
    </main>
  );
}
