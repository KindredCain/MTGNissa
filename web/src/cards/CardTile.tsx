import { useState, type ReactNode } from "react";
import { Link } from "react-router";
import { CardImage } from "./CardImage";
import type { Card } from "./model";
import styles from "./cards.module.css";
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
            ↻
          </button>
        )}
      </div>
      {children}
    </article>
  );
}
export function CardGrid({ cards }: { cards: Card[] }) {
  return (
    <div className={styles.grid}>
      {cards.map((card) => (
        <CardTile key={card.printing_uuid} card={card} />
      ))}
    </div>
  );
}
