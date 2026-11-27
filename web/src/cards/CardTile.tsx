import { useState, type ReactNode, type Ref } from "react";
import { Link } from "react-router";
import { CardImage } from "./CardImage";
import { CardPreview } from "./CardPreview";
import type { Card } from "./model";
import styles from "./cards.module.css";
import { Icon } from "../ui/Icon";
export function CardTile({
  card,
  children,
}: {
  card: Card;
  children?: ReactNode;
}) {
  const [back, setBack] = useState(false);
  return (
    <article className={styles.tile}>
      <CardPreview card={card} back={back}>
        <div className={styles.cardWrap}>
          <Link
            to={`/cards/${encodeURIComponent(card.printing_uuid)}`}
            aria-label={`查看${card.display_name}`}
          >
            <CardImage
              images={card.images}
              name={card.display_name}
              back={back}
            />
          </Link>
          {card.images.has_distinct_back && (
            <button
              className={styles.flip}
              aria-label={`翻面：${card.display_name}`}
              onClick={() => setBack((current) => !current)}
            >
              <Icon name="refresh-cw" size={15} />
            </button>
          )}
        </div>
      </CardPreview>
      {children}
    </article>
  );
}
export function CardGrid({
  cards,
  grid_ref,
}: {
  cards: Card[];
  grid_ref?: Ref<HTMLDivElement>;
}) {
  return (
    <div className={styles.grid} ref={grid_ref}>
      {cards.map((card) => (
        <CardTile key={card.printing_uuid} card={card} />
      ))}
    </div>
  );
}
