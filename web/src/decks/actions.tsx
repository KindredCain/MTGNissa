import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import * as Popover from "@radix-ui/react-popover";
import { api, create_idempotent_post } from "../api/client";
import { Modal, notify_error, useToast } from "../ui/feedback";
import styles from "./decks.module.css";
export function useDeckActions(deck_id: string) {
  const [post_intent] = useState(create_idempotent_post);
  const client = useQueryClient(),
    navigate = useNavigate(),
    toast = useToast();
  const action = useMutation({
    mutationFn: async (kind: "copy" | "delete" | "code") => {
      if (kind === "code") {
        const detail = await client.fetchQuery({
          queryKey: ["decks", "detail", deck_id],
          queryFn: ({ signal }) =>
            api.get("/api/v1/decks/{deck_id}", undefined, {
              path: { deck_id },
              signal,
            }),
        });
        await navigator.clipboard.writeText(detail.source_text);
        toast("卡组代码已复制");
      }
      if (kind === "copy") {
        const copy = await post_intent(
          "/api/v1/decks/{deck_id}/copies",
          undefined,
          { path: { deck_id } },
        );
        void client.invalidateQueries({
          queryKey: ["decks", "list"],
          refetchType: "none",
        });
        navigate(`/decks/${encodeURIComponent(copy.deck_id)}/edit`);
      }
      if (kind === "delete") {
        await api.post("/api/v1/decks/{deck_id}/delete", undefined, {
          path: { deck_id },
        });
        client.removeQueries({ queryKey: ["decks", "detail", deck_id] });
        void client.invalidateQueries({
          queryKey: ["decks"],
          refetchType: "none",
        });
        navigate("/decks");
      }
      if (kind !== "code")
        void client.invalidateQueries({
          queryKey: ["operation_logs"],
          refetchType: "none",
        });
    },
  });
  return action;
}
export function DeleteConfirmation({
  open,
  pending,
  cancel,
  confirm,
}: {
  open: boolean;
  pending: boolean;
  cancel: () => void;
  confirm: () => void;
}) {
  return (
    <Modal open={open} title="删除卡组？">
      <p>删除后无法恢复。</p>
      <p>该卡组记录将被永久删除。</p>
      <div className={styles.confirmActions}>
        <button disabled={pending} onClick={cancel}>
          取消
        </button>
        <button className={styles.danger} disabled={pending} onClick={confirm}>
          删除卡组
        </button>
      </div>
    </Modal>
  );
}
export function DeckMenu({ deck_id }: { deck_id: string }) {
  const [open, setOpen] = useState(false),
    [deleting, setDeleting] = useState(false),
    navigate = useNavigate(),
    action = useDeckActions(deck_id);
  return (
    <div>
      <Popover.Root open={open} onOpenChange={setOpen}>
        <Popover.Trigger aria-label="更多卡组操作">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            aria-hidden="true"
          >
            <circle cx="12" cy="5" r="1" />
            <circle cx="12" cy="12" r="1" />
            <circle cx="12" cy="19" r="1" />
          </svg>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content className={styles.menu} align="end">
            <button
              onClick={() =>
                navigate(`/decks/${encodeURIComponent(deck_id)}/edit`)
              }
            >
              编辑卡组
            </button>
            <button
              disabled={action.isPending}
              onClick={() => {
                setOpen(false);
                action.mutate("copy");
              }}
            >
              复制卡组
            </button>
            <button
              disabled={action.isPending}
              onClick={() => {
                setOpen(false);
                action.mutate("code");
              }}
            >
              复制卡组代码
            </button>
            <button
              className={styles.dangerText}
              onClick={() => {
                setOpen(false);
                setDeleting(true);
              }}
            >
              删除卡组
            </button>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      <DeleteConfirmation
        open={deleting}
        pending={action.isPending}
        cancel={() => setDeleting(false)}
        confirm={() =>
          action.mutate("delete", {
            onSuccess: () => setDeleting(false),
            onError: () => setDeleting(false),
          })
        }
      />
    </div>
  );
}
export async function copy_code(
  source: string,
  toast: (message: string) => void,
) {
  try {
    await navigator.clipboard.writeText(source);
    toast("卡组代码已复制");
  } catch (error) {
    notify_error(error);
  }
}
