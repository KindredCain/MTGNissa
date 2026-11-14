import { useSyncExternalStore } from "react";
// On narrow Chromium viewports a 1280px document can enlarge the layout
// viewport. Fixed overlays must follow the visible viewport instead.
function subscribe(listener: () => void) {
  window.visualViewport?.addEventListener("resize", listener);
  window.visualViewport?.addEventListener("scroll", listener);
  window.addEventListener("resize", listener);
  return () => {
    window.visualViewport?.removeEventListener("resize", listener);
    window.visualViewport?.removeEventListener("scroll", listener);
    window.removeEventListener("resize", listener);
  };
}
function snapshot() {
  const viewport = window.visualViewport;
  return [
    viewport?.offsetLeft ?? 0,
    viewport?.offsetTop ?? 0,
    viewport?.width ?? window.innerWidth,
    viewport?.height ?? window.innerHeight,
  ].join("|");
}
export function useVisualViewport() {
  const [left, top, width, height] = useSyncExternalStore(
    subscribe,
    snapshot,
    () => "0|0|1280|720",
  )
    .split("|")
    .map(Number);
  return { left, top, width, height };
}
