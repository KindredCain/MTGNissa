import * as Popover from "@radix-ui/react-popover";
import * as Checkbox from "@radix-ui/react-checkbox";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useLocation, useSearchParams } from "react-router";
import { useErrorOpen } from "./feedback";
import { Icon, type IconName } from "./Icon";
import styles from "./ui.module.css";
export type Option = { value: string; label: string };
export function usePageGuard() {
  const { pathname } = useLocation(),
    active_page = useRef<object | null>(null);
  useLayoutEffect(() => {
    active_page.current = {};
    return () => {
      active_page.current = null;
    };
  }, [pathname]);
  return () => {
    const page = active_page.current;
    return () => page !== null && active_page.current === page;
  };
}
export function useURLState(
  prepare?: (
    next: URLSearchParams,
    current: URLSearchParams,
  ) => Promise<unknown>,
) {
  const [params, setParams] = useSearchParams();
  const location = useLocation(),
    active_location = useRef<typeof location | null>(location),
    revision = useRef(0);
  useLayoutEffect(() => {
    active_location.current = location;
    revision.current++;
    return () => {
      active_location.current = null;
    };
  }, [location]);
  function update(values: Record<string, string | string[] | undefined>) {
    if (active_location.current !== location) return;
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
      if (intent === revision.current && active_location.current === location) {
        setParams(next, { state: { scroll: "top" } });
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
      <button aria-label="搜索" type="submit">
        <Icon name="search" />
      </button>
      <input
        aria-label={placeholder}
        placeholder={placeholder}
        value={input}
        maxLength={512}
        onChange={(event) => setInput(event.target.value)}
      />
    </form>
  );
}
export function Select({
  label,
  prefix = "",
  value,
  options,
  onChange,
  disabled = false,
  icon,
}: {
  label: string;
  prefix?: string;
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  disabled?: boolean;
  icon?: IconName;
}) {
  return (
    <label className={`${styles.select} ${icon ? styles.selectWithIcon : ""}`}>
      <span className={styles.srOnly}>{label}</span>
      {icon && <Icon name={icon} className={styles.selectLeadingIcon} />}
      <select
        aria-label={label}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {prefix}
            {option.label}
          </option>
        ))}
      </select>
      <Icon name="chevron-down" className={styles.selectChevron} />
    </label>
  );
}
export function MultiSelect({
  label,
  all_label = "全部",
  value,
  options,
  onChange,
}: {
  label: string;
  all_label?: string;
  value: string[];
  options: Option[];
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false),
    [draft, setDraft] = useState(value);
  function all_selected(selected: string[]) {
    return (
      options.length > 0 &&
      selected.length === options.length &&
      options.every((option) => selected.includes(option.value))
    );
  }
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        if (next) setDraft(value);
        else {
          const selected = all_selected(draft) ? [] : draft;
          if (
            selected.length !== value.length ||
            selected.some((item) => !value.includes(item))
          )
            onChange(selected);
        }
        setOpen(next);
      }}
    >
      <Popover.Trigger className={styles.filter} aria-label={label}>
        {label}：
        {!value.length || all_selected(value)
          ? all_label
          : value.length === 1
            ? options.find((option) => option.value === value[0])?.label
            : `已选 ${value.length} 项`}
        <Icon name={open ? "chevron-up" : "chevron-down"} size={13} />
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
              checked={!draft.length || all_selected(draft)}
              onCheckedChange={() => setDraft([])}
              className={styles.checkbox}
            >
              <Checkbox.Indicator>
                <Icon name="check" size={11} />
              </Checkbox.Indicator>
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
                <Checkbox.Indicator>
                  <Icon name="check" size={11} />
                </Checkbox.Indicator>
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
export function SortDivider() {
  return <span className={styles.sortDivider} aria-hidden="true" />;
}
export function LoadMore({
  has_more,
  loading,
  load,
  root,
  failed = false,
  loaded_count = 0,
  end_label = "已加载全部结果",
  more_label = "向下滚动时自动加载更多卡牌",
}: {
  has_more: boolean;
  loading: boolean;
  load: () => void;
  root?: Element | null;
  failed?: boolean;
  loaded_count?: number;
  end_label?: string;
  more_label?: string;
}) {
  const sentinel = useRef<HTMLDivElement>(null),
    requested = useRef(false),
    error_open = useErrorOpen();
  useEffect(() => {
    if (!loading) requested.current = false;
  }, [has_more, loading, root, failed, loaded_count]);
  useEffect(() => {
    const node = sentinel.current;
    if (!node || !has_more || loading || error_open) return;
    function request() {
      if (requested.current) return;
      requested.current = true;
      load();
    }
    if (failed) {
      const scroller = root ?? document.body;
      let armed = false,
        previous_top = scroller.scrollTop;
      function retry() {
        const position = node!.getBoundingClientRect(),
          bounds = root?.getBoundingClientRect();
        if (
          armed &&
          position.top <=
            (bounds?.bottom ?? document.body.clientHeight) + 400 &&
          position.bottom >= (bounds?.top ?? 0) - 400
        )
          request();
      }
      function accepts(event: Event) {
        if (!(event.target instanceof Element)) return false;
        const floating = event.target.closest(
          '[role="dialog"], [role="tooltip"]',
        );
        return !floating || floating.contains(scroller);
      }
      function wheel(event: Event) {
        if (!accepts(event)) return;
        // Modal scroll locks can prevent overscroll while the wheel still expresses retry intent.
        armed = (event as WheelEvent).deltaY > 0;
        retry();
      }
      function keydown(event: Event) {
        if (!accepts(event)) return;
        const key = event as KeyboardEvent,
          target = event.target as Element;
        if (
          key.defaultPrevented ||
          key.altKey ||
          key.ctrlKey ||
          key.metaKey ||
          target.closest('input, textarea, select, [contenteditable="true"]')
        )
          return;
        armed =
          ["ArrowDown", "PageDown", "End"].includes(key.key) ||
          (key.key === " " &&
            !key.shiftKey &&
            !target.closest('button, [role="button"], a'));
        retry();
      }
      function pointerdown(event: Event) {
        armed =
          event.target === scroller && (event as PointerEvent).button === 0;
      }
      function pointerup() {
        armed = false;
      }
      function scroll() {
        const next_top = scroller.scrollTop;
        if (next_top > previous_top) retry();
        else if (next_top < previous_top) armed = false;
        previous_top = next_top;
      }
      scroller.addEventListener("wheel", wheel, { passive: true });
      scroller.addEventListener("keydown", keydown);
      scroller.addEventListener("pointerdown", pointerdown);
      scroller.addEventListener("scroll", scroll, { passive: true });
      document.addEventListener("pointerup", pointerup);
      document.addEventListener("pointercancel", pointerup);
      return () => {
        scroller.removeEventListener("wheel", wheel);
        scroller.removeEventListener("keydown", keydown);
        scroller.removeEventListener("pointerdown", pointerdown);
        scroller.removeEventListener("scroll", scroll);
        document.removeEventListener("pointerup", pointerup);
        document.removeEventListener("pointercancel", pointerup);
      };
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) request();
      },
      { root, rootMargin: "400px" },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [has_more, loading, load, root, failed, error_open]);
  return (
    <div ref={sentinel} className={styles.loadMore}>
      {failed ? null : loading ? (
        <span className={styles.loading}>
          <Icon name="loader-circle" className={styles.spinner} />
          加载中…
        </span>
      ) : has_more ? (
        <span>{more_label}</span>
      ) : loaded_count > 0 ? (
        <span role="status">{end_label}</span>
      ) : null}
    </div>
  );
}
export function Skeleton({ cards = false }: { cards?: boolean }) {
  return (
    <div
      aria-label="加载中"
      data-scroll-loading
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
        <Icon name="minus" />
      </button>
      <output>{value}</output>
      <button
        aria-label={`增加${label}`}
        disabled={disabled}
        onClick={() => onChange(1)}
      >
        <Icon name="plus" />
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
  summary = false,
}: {
  page: number;
  pages: number;
  total: number;
  onChange: (value: number) => void;
  disabled?: boolean;
  summary?: boolean;
}) {
  const start = Math.max(1, Math.min(page - 2, pages - 4));
  return (
    <div className={styles.pagination} data-pagination>
      <span>
        {total
          ? `第 ${(page - 1) * 10 + 1}–${Math.min(page * 10, total)} 条，共 ${total} 条${summary ? "汇总" : ""}`
          : `共 0 条${summary ? "汇总" : ""}`}
      </span>
      <nav aria-label="分页">
        <button
          disabled={disabled || page <= 1}
          onClick={() => onChange(page - 1)}
          aria-label="上一页"
        >
          <Icon name="chevron-left" />
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
          <Icon name="chevron-right" />
        </button>
      </nav>
    </div>
  );
}
