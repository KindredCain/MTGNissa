import { useState } from "react";
import type { DTO } from "../api/client";
import { Icon } from "../ui/Icon";
import styles from "./cards.module.css";

export function CardImagePlaceholder({ name }: { name: string }) {
  return (
    <div
      className={styles.placeholder}
      role="img"
      aria-label={`${name}，暂无卡图`}
    >
      <Icon name="sparkles" size={46} />
    </div>
  );
}

export function CardImage({
  images,
  name,
  back = false,
  className = "",
}: {
  images: DTO<"CardImagesResponse">;
  name: string;
  back?: boolean;
  className?: string;
}) {
  const [failed, setFailed] = useState<string | null>(null);
  const url =
    back && images.has_distinct_back ? images.back_url : images.front_url;
  return (
    <div className={`${styles.image} ${className}`}>
      {url && url !== failed ? (
        <img
          src={url}
          alt={name}
          loading="lazy"
          decoding="async"
          draggable={false}
          onError={() => setFailed(url)}
        />
      ) : (
        <CardImagePlaceholder name={name} />
      )}
    </div>
  );
}
