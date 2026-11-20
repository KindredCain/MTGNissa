import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

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
    return [rect.x, rect.y, rect.width, rect.height].join("|");
  });
  if (snapshot === null) return null;
  const [x, y, width, height] = snapshot.split("|").map(Number);
  return new DOMRect(x, y, width, height);
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
