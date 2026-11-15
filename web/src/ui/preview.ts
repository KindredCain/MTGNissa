import { useEffect, useRef, useState } from "react";

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
