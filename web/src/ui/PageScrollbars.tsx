import { useEffect, useState, type CSSProperties } from "react";
import { useVisualViewport } from "./viewport";
import styles from "./page-scrollbars.module.css";

export function PageScrollbars() {
  const viewport = useVisualViewport();
  const [scroll, setScroll] = useState({
    left: 0,
    top: 0,
    width: 0,
    height: 0,
    max_x: 0,
    max_y: 0,
    locked: false,
  });
  useEffect(() => {
    const body = document.body;
    let frame = 0;
    function schedule() {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        setScroll({
          left: body.scrollLeft,
          top: body.scrollTop,
          width: body.clientWidth,
          height: body.clientHeight,
          max_x: Math.max(0, body.scrollWidth - body.clientWidth),
          max_y: Math.max(0, body.scrollHeight - body.clientHeight),
          locked:
            body.hasAttribute("data-scroll-locked") ||
            ["hidden", "clip"].includes(getComputedStyle(body).overflowY),
        });
      });
    }
    const resize = new ResizeObserver(schedule);
    resize.observe(body);
    resize.observe(document.getElementById("root")!);
    const lock = new MutationObserver(schedule);
    lock.observe(body, {
      attributes: true,
      attributeFilter: ["style", "data-scroll-locked"],
    });
    body.addEventListener("scroll", schedule, { passive: true });
    schedule();
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      lock.disconnect();
      body.removeEventListener("scroll", schedule);
    };
  }, []);

  if (scroll.locked) return null;
  return (
    <>
      {scroll.max_y > 0 && (
        <Scrollbar
          vertical
          maximum={scroll.max_y}
          position={scroll.top}
          viewport_size={scroll.height}
          length={viewport.height - 4 - (scroll.max_x > 0 ? 12 : 0)}
          style={{
            left: viewport.left + viewport.width - 12,
            top: viewport.top + 2,
          }}
        />
      )}
      {scroll.max_x > 0 && (
        <Scrollbar
          maximum={scroll.max_x}
          position={scroll.left}
          viewport_size={scroll.width}
          length={viewport.width - 4 - (scroll.max_y > 0 ? 12 : 0)}
          style={{
            left: viewport.left + 2,
            top: viewport.top + viewport.height - 12,
          }}
        />
      )}
    </>
  );
}

function Scrollbar({
  vertical = false,
  maximum,
  position,
  viewport_size,
  length,
  style,
}: {
  vertical?: boolean;
  maximum: number;
  position: number;
  viewport_size: number;
  length: number;
  style: CSSProperties;
}) {
  const value = Math.max(0, Math.min(maximum, position));
  const thumb = Math.min(
    length,
    Math.max(28, (length * viewport_size) / (viewport_size + maximum)),
  );
  function move(next: number, forward = next > document.body.scrollTop) {
    if (vertical) {
      document.body.scrollTop = next;
      if (forward)
        document.body.dispatchEvent(new Event("scrollbar-scroll-down"));
    } else document.body.scrollLeft = next;
  }
  return (
    <input
      className={`${styles.scrollbar} ${vertical ? styles.vertical : styles.horizontal}`}
      style={
        {
          ...style,
          [vertical ? "height" : "width"]: length,
          "--thumb-size": `${thumb}px`,
        } as CSSProperties
      }
      type="range"
      role="scrollbar"
      aria-label={vertical ? "页面纵向滚动" : "页面横向滚动"}
      aria-controls="root"
      aria-orientation={vertical ? "vertical" : "horizontal"}
      aria-valuemin={0}
      aria-valuemax={maximum}
      aria-valuenow={value}
      min={0}
      max={maximum}
      step="any"
      value={value}
      onChange={(event) => move(Number(event.target.value))}
      onKeyDown={(event) => {
        const changes: Record<string, number> = {
          Home: -value,
          End: maximum - value,
          PageUp: -viewport_size,
          PageDown: viewport_size,
          [vertical ? "ArrowUp" : "ArrowLeft"]: -40,
          [vertical ? "ArrowDown" : "ArrowRight"]: 40,
        };
        const change = changes[event.key];
        if (change === undefined) return;
        event.preventDefault();
        move(
          value + change,
          ["End", "PageDown", "ArrowDown"].includes(event.key),
        );
      }}
    />
  );
}
