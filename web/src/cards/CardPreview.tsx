import {
  createContext,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type DTO } from "../api/client";
import { useHoverPreview } from "../ui/preview";
import { CardFaceText, CardTextPreview } from "./CardTextPreview";
import type { Card } from "./model";

const PreviewScope = createContext<{
  active: string | null;
  activate: (id: string) => void;
} | null>(null);

export function CardPreviewScope({ children }: { children: ReactNode }) {
  const [active, activate] = useState<string | null>(null);
  const value = useMemo(() => ({ active, activate }), [active]);
  return (
    <PreviewScope.Provider value={value}>{children}</PreviewScope.Provider>
  );
}

export function CardPreview({
  card,
  back,
  children,
}: {
  card: Card;
  back: boolean;
  children: ReactNode;
}) {
  const scope = useContext(PreviewScope),
    id = useId();
  const {
    value: anchor,
    show,
    keep,
    leave,
    hide,
  } = useHoverPreview<HTMLElement>();
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  function cancel_open() {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  }
  function open(element: HTMLElement) {
    cancel_open();
    keep();
    if (anchor === element && scope?.active === id) return;
    timer.current = setTimeout(() => {
      timer.current = null;
      show(element);
      scope?.activate(id);
    }, 300);
  }
  function close() {
    cancel_open();
    leave();
  }
  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );
  return (
    <>
      <div
        onPointerEnter={(event) => open(event.currentTarget)}
        onPointerLeave={close}
        onFocusCapture={(event) => open(event.currentTarget)}
        onBlurCapture={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget)) close();
        }}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            cancel_open();
            hide();
          }
        }}
      >
        {children}
      </div>
      {anchor &&
        scope?.active === id &&
        createPortal(
          <CardTextPreview
            anchor={anchor}
            onPointerEnter={keep}
            onPointerLeave={close}
            onFocusCapture={keep}
            onBlurCapture={close}
            onKeyDown={(event) => {
              if (event.key === "Escape") hide();
            }}
          >
            <PreviewContent card={card} back={back} />
          </CardTextPreview>,
          document.body,
        )}
    </>
  );
}

function PreviewContent({ card, back }: { card: Card; back: boolean }) {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: ["card_preview", card.printing_uuid],
    queryFn: ({ signal }) =>
      api.get("/api/v1/cards/{printing_uuid}/preview", undefined, {
        path: { printing_uuid: card.printing_uuid },
        signal,
      }),
    initialData: (): DTO<"CardPreviewResponse"> | undefined => {
      const detail = client.getQueryData<DTO<"CardDetailResponse">>([
        "card",
        card.printing_uuid,
      ]);
      return detail ? { faces: detail.faces } : undefined;
    },
    initialDataUpdatedAt: () =>
      client.getQueryState(["card", card.printing_uuid])?.dataUpdatedAt,
    staleTime: 5 * 60_000,
    gcTime: 5 * 60_000,
  });
  return (
    <>
      {query.data ? (
        <CardFaceText
          faces={query.data.faces}
          back={back}
          distinct_back={card.images.has_distinct_back}
          fallback_name={card.display_name}
        />
      ) : query.isPending ? (
        <p role="status">正在加载说明…</p>
      ) : (
        <p>说明加载失败</p>
      )}
    </>
  );
}
