import type { Ref } from "react";
import { useVisualViewport } from "../ui/viewport";
import styles from "./decks.module.css";
import { Icon } from "../ui/Icon";
export function Bookmark({
  top,
  disabled = false,
  hidden = false,
  button_ref,
  onClick,
}: {
  top: number;
  disabled?: boolean;
  hidden?: boolean;
  button_ref?: Ref<HTMLButtonElement>;
  onClick: () => void;
}) {
  const viewport = useVisualViewport();
  return (
    <button
      ref={button_ref}
      hidden={hidden}
      className={styles.drawerButton}
      style={{
        top: viewport.top + top,
        left: viewport.left + viewport.width - 38,
        right: "auto",
      }}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon name="chart-no-axes-column-increasing" size={15} />
      <span>卡组统计</span>
    </button>
  );
}
