import { useVisualViewport } from "../ui/viewport";
import styles from "./decks.module.css";
export function Bookmark({
  top,
  disabled = false,
  onClick,
}: {
  top: number;
  disabled?: boolean;
  onClick: () => void;
}) {
  const viewport = useVisualViewport();
  return (
    <button
      className={styles.drawerButton}
      style={{
        top: viewport.top + top,
        left: viewport.left + viewport.width - 38,
        right: "auto",
      }}
      disabled={disabled}
      onClick={onClick}
    >
      卡组统计
    </button>
  );
}
