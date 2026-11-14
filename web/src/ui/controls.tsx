import * as Popover from "@radix-ui/react-popover";
import * as Checkbox from "@radix-ui/react-checkbox";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router";
import styles from "./ui.module.css";
export type Option = { value: string; label: string };
export function useURLState(
  prepare?: (
    next: URLSearchParams,
    current: URLSearchParams,
  ) => Promise<unknown>,
) {
  const [params, setParams] = useSearchParams();
  const revision = useRef(0);
  function update(values: Record<string, string | string[] | undefined>) {
    const next = new URLSearchParams(params);
    next.delete("cursor");
    next.delete("page");
    for (const [key, value] of Object.entries(values)) {
      next.delete(key);
      for (const item of Array.isArray(value) ? value : value ? [value] : [])
        next.append(key, item);
    }
    const intent = ++revision.current;
    const commit = () => {
      if (intent === revision.current) {
        setParams(next);
        document.body.scrollTo({ top: 0 });
      }
    };
    if (prepare)
      void prepare(next, params)
        .then(commit)
        .catch(() => {});
    else commit();
  }
  return { params, update };
}
export function SearchInput({
  value,
  onSubmit,
  placeholder = "按名称搜索",
}: {
  value: string;
  onSubmit: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <SearchField
      key={value}
      value={value}
      onSubmit={onSubmit}
      placeholder={placeholder}
    />
  );
}
function SearchField({
  value,
  onSubmit,
  placeholder,
}: {
  value: string;
  onSubmit: (value: string) => void;
  placeholder: string;
}) {
  const [input, setInput] = useState(value);
  return (
    <form
      className={styles.search}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(input.trim());
      }}
    >
      <input
        aria-label={placeholder}
        placeholder={placeholder}
        value={input}
        maxLength={512}
        onChange={(event) => setInput(event.target.value)}
      />
      <button aria-label="搜索" type="submit">
        ⌕
      </button>
    </form>
  );
}
export function Select({
  label,
  value,
  options,
  onChange,
  disabled = false,
}: {
  label: string;
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <label className={styles.select}>
      <span className={styles.srOnly}>{label}</span>
      <select
        aria-label={label}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}
export function MultiSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string[];
  options: Option[];
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false),
    [draft, setDraft] = useState(value);
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        if (next) setDraft(value);
        else if (
          draft.length !== value.length ||
          draft.some((item) => !value.includes(item))
        )
          onChange(draft);
        setOpen(next);
      }}
    >
      <Popover.Trigger className={styles.filter} aria-label={label}>
        {label}：
        {!value.length || value.length === options.length
          ? "全部"
          : value.length === 1
            ? options.find((option) => option.value === value[0])?.label
            : `已选 ${value.length} 项`}
        <span>⌄</span>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          className={styles.popover}
          align="start"
          sideOffset={6}
        >
          <label className={styles.check}>
            <Checkbox.Root
              aria-label={`${label}：全部`}
              checked={!draft.length}
              onCheckedChange={() => setDraft([])}
              className={styles.checkbox}
            >
              <Checkbox.Indicator>✓</Checkbox.Indicator>
            </Checkbox.Root>
            全部
          </label>
          {options.map((option) => (
            <label key={option.value} className={styles.check}>
              <Checkbox.Root
                aria-label={option.label}
                className={styles.checkbox}
                checked={draft.includes(option.value)}
                onCheckedChange={(checked) =>
                  setDraft((current) =>
                    checked
                      ? [...current, option.value]
                      : current.filter((item) => item !== option.value),
                  )
                }
              >
                <Checkbox.Indicator>✓</Checkbox.Indicator>
              </Checkbox.Root>
              {option.label}
            </label>
          ))}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
export function Toolbar({
  children,
  count,
}: {
  children: ReactNode;
  count?: ReactNode;
}) {
  return (
    <div className={styles.toolbar} data-toolbar>
      {children}
      <span className={styles.count}>{count}</span>
    </div>
  );
}
export function LoadMore({
  has_more,
  loading,
  load,
  root,
  failed = false,
}: {
  has_more: boolean;
  loading: boolean;
  load: () => void;
  root?: Element | null;
  failed?: boolean;
}) {
  const sentinel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!sentinel.current || !has_more || loading || failed) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) load();
      },
      { root, rootMargin: "400px" },
    );
    observer.observe(sentinel.current);
    return () => observer.disconnect();
  }, [has_more, loading, load, root, failed]);
  return (
    <div ref={sentinel} className={styles.loadMore}>
      {failed ? null : loading ? (
        "加载中…"
      ) : has_more ? (
        <button onClick={load}>加载更多</button>
      ) : null}
    </div>
  );
}
export function Skeleton({ cards = false }: { cards?: boolean }) {
  return (
    <div
      aria-label="加载中"
      className={cards ? styles.skeletonGrid : styles.skeletonRows}
    >
      {Array.from({ length: 10 }, (_, index) => (
        <div key={index} />
      ))}
    </div>
  );
}
export function Stepper({
  label,
  value,
  disabled,
  decrease_disabled,
  onChange,
}: {
  label: string;
  value: number;
  disabled: boolean;
  decrease_disabled?: boolean;
  onChange: (delta: 1 | -1) => void;
}) {
  return (
    <div className={styles.stepper}>
      <span>{label}</span>
      <button
        aria-label={`减少${label}`}
        disabled={disabled || decrease_disabled || value === 0}
        onClick={() => onChange(-1)}
      >
        −
      </button>
      <output>{value}</output>
      <button
        aria-label={`增加${label}`}
        disabled={disabled}
        onClick={() => onChange(1)}
      >
        ＋
      </button>
    </div>
  );
}
export function Pagination({
  page,
  pages,
  total,
  onChange,
  disabled,
}: {
  page: number;
  pages: number;
  total: number;
  onChange: (value: number) => void;
  disabled?: boolean;
}) {
  const start = Math.max(1, Math.min(page - 2, pages - 4));
  return (
    <div className={styles.pagination}>
      <span>
        {total
          ? `第 ${(page - 1) * 10 + 1}–${Math.min(page * 10, total)} 条，共 ${total} 条`
          : "共 0 条"}
      </span>
      <nav aria-label="分页">
        <button
          disabled={disabled || page <= 1}
          onClick={() => onChange(page - 1)}
          aria-label="上一页"
        >
          ‹
        </button>
        {Array.from(
          { length: Math.max(0, Math.min(pages, 5)) },
          (_, index) => start + index,
        ).map((index) => (
          <button
            key={index}
            aria-current={index === page ? "page" : undefined}
            disabled={disabled}
            onClick={() => onChange(index)}
          >
            {index}
          </button>
        ))}
        <button
          disabled={disabled || page >= pages}
          onClick={() => onChange(page + 1)}
          aria-label="下一页"
        >
          ›
        </button>
      </nav>
    </div>
  );
}
