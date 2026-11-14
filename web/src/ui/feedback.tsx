import * as Dialog from "@radix-ui/react-dialog";
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { ApiError } from "../api/client";
import styles from "./ui.module.css";
import { useVisualViewport } from "./viewport";
const events = new EventTarget();
export function notify_error(error: unknown) {
  if (error instanceof DOMException && error.name === "AbortError") return;
  events.dispatchEvent(
    new CustomEvent("error", {
      detail:
        error instanceof ApiError
          ? error.message
          : "发生意外错误，操作未完成。请稍后重试。",
    }),
  );
}
export function validation_error(message: string) {
  notify_error(new ApiError("validation_failed", message));
}
const ToastContext = createContext<(message: string) => void>(() => {});
export function useToast() {
  return useContext(ToastContext);
}
export function Modal({
  open,
  title,
  children,
  close,
  className,
  close_label = "关闭",
  dismiss = false,
}: {
  open: boolean;
  title: string;
  children: ReactNode;
  close?: () => void;
  className?: string;
  close_label?: string;
  dismiss?: boolean;
}) {
  const viewport = useVisualViewport();
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(value) => {
        if (!value && dismiss) close?.();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay
          className={styles.overlay}
          style={{
            left: viewport.left,
            top: viewport.top,
            width: viewport.width,
            height: viewport.height,
            right: "auto",
            bottom: "auto",
          }}
        />
        <Dialog.Content
          aria-describedby={undefined}
          className={`${styles.modal} ${className ?? ""}`}
          style={{
            left: viewport.left + viewport.width / 2,
            top: viewport.top + viewport.height / 2,
            maxWidth: viewport.width - 32,
            maxHeight: viewport.height - 64,
          }}
          onEscapeKeyDown={(event) => {
            if (!dismiss) event.preventDefault();
          }}
          onPointerDownOutside={(event) => event.preventDefault()}
        >
          <div className={styles.modalTitle} data-modal-title>
            <Dialog.Title>{title}</Dialog.Title>
            {close && <button onClick={close}>{close_label}</button>}
          </div>
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [errors, setErrors] = useState<string[]>([]);
  const [toast, setToast] = useState("");
  useEffect(() => {
    const handler = (event: Event) =>
      setErrors((current) => [
        ...current,
        (event as CustomEvent<string>).detail,
      ]);
    events.addEventListener("error", handler);
    return () => events.removeEventListener("error", handler);
  }, []);
  useEffect(() => {
    if (toast) {
      const id = setTimeout(() => setToast(""), 2500);
      return () => clearTimeout(id);
    }
  }, [toast]);
  return (
    <ToastContext.Provider value={setToast}>
      {children}
      <Modal open={errors.length > 0} title="操作未完成">
        <div className={styles.errorBody}>{errors[0]}</div>
        <div className={styles.modalFooter}>
          <button onClick={() => setErrors((current) => current.slice(1))}>
            关闭
          </button>
        </div>
      </Modal>
      {toast && (
        <div role="status" className={styles.toast}>
          {toast}
        </div>
      )}
    </ToastContext.Provider>
  );
}
