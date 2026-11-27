import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { useVisualViewport } from "./viewport";

export function useAnchorRect(anchor: HTMLElement) {
  const connected = anchor.isConnected;
  const subscribe = useCallback(
    (notify: () => void) => {
      if (!connected) return () => {};
      let frame: number | undefined;
      function update() {
        if (frame !== undefined) return;
        frame = requestAnimationFrame(() => {
          frame = undefined;
          notify();
        });
      }
      const observer = new ResizeObserver(update);
      for (
        let element: HTMLElement | null = anchor;
        element;
        element = element.parentElement
      )
        observer.observe(element);
      document
        .querySelectorAll("[data-preview-header], [data-toolbar]")
        .forEach((element) => observer.observe(element));
      document.addEventListener("scroll", update, true);
      window.addEventListener("resize", update);
      window.visualViewport?.addEventListener("resize", update);
      window.visualViewport?.addEventListener("scroll", update);
      return () => {
        if (frame !== undefined) cancelAnimationFrame(frame);
        observer.disconnect();
        document.removeEventListener("scroll", update, true);
        window.removeEventListener("resize", update);
        window.visualViewport?.removeEventListener("resize", update);
        window.visualViewport?.removeEventListener("scroll", update);
      };
    },
    [anchor, connected],
  );
  const snapshot = useSyncExternalStore(subscribe, () => {
    if (!anchor.isConnected) return null;
    const rect = anchor.getBoundingClientRect();
    const headers = Array.from(
      document.querySelectorAll("[data-preview-header], [data-toolbar]"),
      (element) => {
        const bounds = element.getBoundingClientRect();
        return [bounds.x, bounds.y, bounds.width, bounds.height].join("|");
      },
    );
    return [
      [rect.x, rect.y, rect.width, rect.height].join("|"),
      ...headers,
    ].join(";");
  });
  if (snapshot === null) return null;
  const [x, y, width, height] = snapshot.split(";")[0].split("|").map(Number);
  return new DOMRect(x, y, width, height);
}

export function usePreviewBounds(anchor: HTMLElement) {
  const rect = useAnchorRect(anchor),
    viewport = useVisualViewport();
  const bounds = {
    left: viewport.left + 8,
    top: viewport.top + 8,
    right: viewport.left + viewport.width - 8,
    bottom: viewport.top + viewport.height - 8,
  };
  // Modal previews float above the page headers and keep their existing viewport.
  if (!anchor.closest('[role="dialog"]')) {
    for (const header of document.querySelectorAll(
      "[data-preview-header], [data-toolbar]",
    )) {
      const area = header.getBoundingClientRect();
      if (
        area.width &&
        area.height &&
        area.top < bounds.bottom &&
        area.bottom > viewport.top
      )
        bounds.top = Math.max(bounds.top, area.bottom + 8);
    }
  }
  let visible = rect;
  if (rect) {
    let left = Math.max(rect.left, bounds.left),
      right = Math.min(rect.right, bounds.right),
      top = Math.max(rect.top, bounds.top),
      bottom = Math.min(rect.bottom, bounds.bottom);
    for (
      let parent = anchor.parentElement;
      parent;
      parent = parent.parentElement
    ) {
      const style = getComputedStyle(parent),
        area = parent.getBoundingClientRect();
      if (/auto|scroll|hidden|clip/.test(style.overflowX)) {
        left = Math.max(left, area.left + parent.clientLeft);
        right = Math.min(
          right,
          area.left + parent.clientLeft + parent.clientWidth,
        );
      }
      if (/auto|scroll|hidden|clip/.test(style.overflowY)) {
        top = Math.max(top, area.top + parent.clientTop);
        bottom = Math.min(
          bottom,
          area.top + parent.clientTop + parent.clientHeight,
        );
      }
    }
    if (left >= right || top >= bottom) visible = null;
  }
  return { rect: visible, bounds, viewport };
}

export function useHoverPreview<T>(fallback?: () => T | null) {
  const [value, setValue] = useState<T | null>(null),
    timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  function keep() {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  }
  function show(next: T | null) {
    keep();
    setValue(next);
  }
  function hide() {
    show(null);
  }
  function leave() {
    keep();
    timer.current = setTimeout(() => {
      timer.current = null;
      setValue(fallback?.() ?? null);
    }, 150);
  }
  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );
  return { value, show, keep, leave, hide };
}
