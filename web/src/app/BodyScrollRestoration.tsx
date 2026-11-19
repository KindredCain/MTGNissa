import { useLayoutEffect, useRef, useState } from "react";
import { useLocation, useNavigationType } from "react-router";

type Position = { top: number; left: number };
const storage_key = "mtgnissa:body-scroll";

function read_positions(): Map<string, Position> {
  try {
    return new Map(JSON.parse(sessionStorage.getItem(storage_key) ?? "[]"));
  } catch {
    return new Map();
  }
}

export function BodyScrollRestoration() {
  const location = useLocation(),
    navigation_type = useNavigationType();
  const [positions] = useState(read_positions);
  const previous = useRef(location),
    active_key = useRef(location.key),
    restoring = useRef(false),
    cancel_restore = useRef<() => void>(() => {});

  useLayoutEffect(() => {
    const original = history.scrollRestoration;
    history.scrollRestoration = "manual";
    function remember() {
      if (!restoring.current)
        positions.set(active_key.current, {
          top: document.body.scrollTop,
          left: document.body.scrollLeft,
        });
    }
    function persist() {
      remember();
      try {
        sessionStorage.setItem(storage_key, JSON.stringify([...positions]));
      } catch {
        /* A full or unavailable session store must not block navigation. */
      }
    }
    document.body.addEventListener("scroll", remember, { passive: true });
    window.addEventListener("pagehide", persist);
    return () => {
      persist();
      cancel_restore.current();
      history.scrollRestoration = original;
      document.body.removeEventListener("scroll", remember);
      window.removeEventListener("pagehide", persist);
    };
  }, [positions]);

  useLayoutEffect(() => {
    cancel_restore.current();
    const from = previous.current;
    previous.current = location;
    active_key.current = location.key;
    const policy = (location.state as { scroll?: string } | null)?.scroll;
    const target =
      navigation_type === "POP"
        ? (positions.get(location.key) ?? { top: 0, left: 0 })
        : policy === "preserve" ||
            (policy !== "top" && from.pathname === location.pathname)
          ? { top: document.body.scrollTop, left: document.body.scrollLeft }
          : { top: 0, left: 0 };
    positions.set(location.key, target);
    restoring.current = true;
    let observer: ResizeObserver | undefined;
    let mutations: MutationObserver | undefined;
    let stopped = false;
    const events = ["wheel", "touchstart", "pointerdown", "keydown"] as const;
    function cleanup() {
      stopped = true;
      observer?.disconnect();
      mutations?.disconnect();
      restoring.current = false;
      for (const event of events)
        document.removeEventListener(event, finish, true);
    }
    function finish() {
      cleanup();
      positions.set(location.key, {
        top: document.body.scrollTop,
        left: document.body.scrollLeft,
      });
    }
    function restore() {
      if (stopped) return;
      document.body.scrollTo({ ...target, behavior: "instant" });
      if (
        Math.abs(document.body.scrollTop - target.top) < 1 &&
        Math.abs(document.body.scrollLeft - target.left) < 1 &&
        !document.querySelector("[data-scroll-loading]")
      )
        finish();
    }
    cancel_restore.current = cleanup;
    for (const event of events)
      document.addEventListener(event, finish, {
        capture: true,
        passive: true,
      });
    restore();
    if (restoring.current) {
      const root = document.getElementById("root") ?? document.body;
      observer = new ResizeObserver(restore);
      observer.observe(root);
      mutations = new MutationObserver(restore);
      mutations.observe(root, { childList: true, subtree: true });
    }
    return cleanup;
  }, [location, navigation_type, positions]);
  return null;
}
