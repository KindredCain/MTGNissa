import {
  Fragment,
  useEffect,
  useState,
  type CSSProperties,
  type HTMLAttributes,
} from "react";
import type { DTO } from "../api/client";
import { usePreviewBounds } from "../ui/preview";
import styles from "./preview.module.css";

export function CardFaceText({
  faces,
  back = false,
  distinct_back = false,
  fallback_name = "",
}: {
  faces: DTO<"CardPreviewFaceResponse">[];
  back?: boolean;
  distinct_back?: boolean;
  fallback_name?: string;
}) {
  const active = faces[back ? 1 : 0] ?? faces[0];
  const displayed = distinct_back ? (active ? [active] : []) : faces;
  return displayed.map((face) => (
    <Fragment key={face.face_index}>
      <h3>{face.display_name || face.name || fallback_name}</h3>
      <div>
        <small>类别</small>
        <p>{face.display_type_line ?? face.type_line}</p>
      </div>
      <div>
        <small>规则文字</small>
        <p>{face.display_oracle_text ?? face.oracle_text}</p>
      </div>
    </Fragment>
  ));
}

export function CardTextPreview({
  anchor,
  children,
  onBlurCapture,
  ...attributes
}: HTMLAttributes<HTMLDivElement> & { anchor: HTMLElement }) {
  const { rect, bounds } = usePreviewBounds(anchor);
  const [element, setElement] = useState<HTMLDivElement | null>(null),
    [height, setHeight] = useState(164);
  useEffect(() => {
    if (!element) return;
    const observer = new ResizeObserver(() => setHeight(element.offsetHeight));
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);
  const max_height = Math.min(360, bounds.bottom - bounds.top);
  if (!rect || max_height < 48) return null;
  const width = Math.min(300, bounds.right - bounds.left);
  const on_left = rect.right + 6 + width > bounds.right;
  const left = Math.max(
    bounds.left,
    Math.min(
      on_left ? rect.left - width - 6 : rect.right + 6,
      bounds.right - width,
    ),
  );
  const top = Math.max(
    bounds.top,
    Math.min(rect.top, bounds.bottom - Math.min(height, max_height)),
  );
  return (
    <div
      {...attributes}
      ref={setElement}
      role="tooltip"
      data-card-preview
      data-side={on_left ? "left" : "right"}
      className={styles.bubble}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget))
          onBlurCapture?.(event);
      }}
      style={
        {
          left,
          top,
          width,
          "--preview-max-height": `${max_height}px`,
          "--arrow-top": `${Math.max(16, Math.min(Math.min(height, max_height) - 16, rect.top + 48 - top))}px`,
        } as CSSProperties
      }
    >
      <div className={styles.content} tabIndex={0} aria-label="卡牌说明">
        {children}
      </div>
    </div>
  );
}
