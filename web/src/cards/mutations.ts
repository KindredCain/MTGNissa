import { useRef } from "react";
import {
  useMutation,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";
import { api, type DTO } from "../api/client";
import type { WishlistPageView } from "../api/lists";
type CollectionPage = DTO<"CollectionPageResponse">;
export function useCardActions(printing_uuid: string) {
  const client = useQueryClient(),
    gate = useRef(false);
  const mutation = useMutation({
    mutationFn: async (
      intent:
        { finish: "normal" | "foil"; delta: 1 | -1 } | { included: boolean },
    ) => {
      const options = {
        path: { printing_uuid },
        idempotency_key: crypto.randomUUID(),
      };
      if ("included" in intent) {
        const state = await api.post(
          "/api/v1/cards/{printing_uuid}/wishlist_state",
          intent,
          options,
        );
        client.setQueriesData<InfiniteData<WishlistPageView>>(
          { queryKey: ["wishlist", "list"] },
          (current) =>
            current && {
              ...current,
              pages: current.pages.map((page) => ({
                ...page,
                items: page.items.map((item) =>
                  item.card.printing_uuid === printing_uuid
                    ? { ...item, included: state.included }
                    : item,
                ),
              })),
            },
        );
        client.setQueryData<DTO<"CardDetailResponse">>(
          ["card", printing_uuid],
          (current) =>
            current && { ...current, wishlist: { included: state.included } },
        );
        void client.invalidateQueries({
          queryKey: ["wishlist", "options"],
          refetchType: "none",
        });
      } else {
        const state = await api.post(
          "/api/v1/cards/{printing_uuid}/collection_adjustments",
          intent,
          options,
        );
        client.setQueriesData<InfiniteData<CollectionPage>>(
          { queryKey: ["collection", "list"] },
          (current) =>
            current && {
              ...current,
              pages: current.pages.map((page) => ({
                ...page,
                summary: state.collection,
                items: page.items.map((item) => ({
                  ...item,
                  ...(!item.merged_sets &&
                  !item.merged_languages &&
                  item.card.printing_uuid === printing_uuid
                    ? {
                        normal_quantity: state.group.normal_quantity,
                        foil_quantity: state.group.foil_quantity,
                      }
                    : {}),
                  group_summary:
                    item.group_set?.code === state.set_summary?.set_code &&
                    state.set_summary
                      ? state.set_summary
                      : item.group_summary,
                })),
              })),
            },
        );
        client.setQueryData<DTO<"CardDetailResponse">>(
          ["card", printing_uuid],
          (current) => current && { ...current, collection: state.group },
        );
        void client.invalidateQueries({ queryKey: ["card", printing_uuid] });
        void client.invalidateQueries({
          queryKey: ["collection", "options"],
          refetchType: "none",
        });
      }
      void client.invalidateQueries({
        queryKey: ["decks"],
        refetchType: "none",
      });
      void client.invalidateQueries({
        queryKey: ["operation_logs"],
        refetchType: "none",
      });
    },
    onSettled: () => {
      gate.current = false;
    },
  });
  function act(intent: Parameters<typeof mutation.mutate>[0]) {
    if (gate.current) return;
    gate.current = true;
    mutation.mutate(intent);
  }
  return { pending: mutation.isPending, act };
}
