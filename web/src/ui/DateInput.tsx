import { useId, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { DayPicker, type ChevronProps } from "@daypicker/react";
import { zhCN } from "@daypicker/react/locale";
import { Icon } from "./Icon";
import { format_date, parse_date } from "./date";
import styles from "./date-input.module.css";

function Chevron({ orientation, className }: ChevronProps) {
  const icons = {
    left: "chevron-left",
    right: "chevron-right",
    up: "chevron-up",
    down: "chevron-down",
  } as const;
  return (
    <Icon
      name={icons[orientation ?? "right"]}
      className={className}
      size={14}
    />
  );
}

export function DateInput({
  label,
  value,
  onChange,
  range,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  range?: { start: string; end: string; target: "start" | "end" };
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [month, setMonth] = useState(() => parse_date(value) ?? new Date());
  const [preview, setPreview] = useState<Date>();
  const start = parse_date(range?.start ?? "");
  const end = parse_date(range?.end ?? "");
  const anchor = range?.target === "end" ? start : end;
  const candidate =
    range &&
    preview &&
    (!anchor ||
      (range.target === "end"
        ? format_date(preview) >= format_date(anchor)
        : format_date(preview) <= format_date(anchor)))
      ? preview
      : undefined;
  const from = range?.target === "start" ? (candidate ?? start) : start;
  const to = range?.target === "end" ? (candidate ?? end) : end;
  const complete = from && to && format_date(from) <= format_date(to);
  const single = complete && format_date(from) === format_date(to);
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        if (next) setMonth(parse_date(value) ?? anchor ?? new Date());
        setPreview(undefined);
        setOpen(next);
      }}
    >
      <Popover.Anchor asChild>
        <div className={styles.field}>
          <label htmlFor={id}>{label}</label>
          <input
            id={id}
            type="text"
            value={value}
            placeholder="YYYY-MM-DD"
            autoComplete="off"
            inputMode="numeric"
            maxLength={10}
            onChange={(event) => onChange(event.target.value)}
          />
          <Popover.Trigger
            className={styles.trigger}
            aria-label={`选择${label}`}
          >
            <Icon name="calendar-days" />
          </Popover.Trigger>
        </div>
      </Popover.Anchor>
      <Popover.Portal>
        <Popover.Content
          className={styles.popover}
          align="start"
          sideOffset={6}
          collisionPadding={8}
          aria-label={`${label}日历`}
          onOpenAutoFocus={(event) => event.preventDefault()}
          onPointerLeave={() => setPreview(undefined)}
        >
          <DayPicker
            mode="single"
            required
            autoFocus
            locale={zhCN}
            month={month}
            onMonthChange={(next) => {
              setMonth(next);
              setPreview(undefined);
            }}
            selected={candidate ?? parse_date(value) ?? anchor}
            onDayMouseEnter={range ? (day) => setPreview(day) : undefined}
            onDayFocus={range ? (day) => setPreview(day) : undefined}
            modifiers={{
              interval_anchor: anchor,
              interval_start: complete ? from : undefined,
              interval_end: complete ? to : undefined,
              interval_middle:
                complete && !single ? { after: from, before: to } : undefined,
              interval_single: single ? from : undefined,
            }}
            modifiersClassNames={{
              interval_anchor: styles.rangeAnchor,
              interval_start: styles.rangeStart,
              interval_end: styles.rangeEnd,
              interval_middle: styles.rangeMiddle,
              interval_single: styles.rangeSingle,
            }}
            showOutsideDays
            fixedWeeks
            onSelect={(date) => {
              onChange(format_date(date));
              setPreview(undefined);
              setOpen(false);
            }}
            components={{ Chevron }}
            labels={{
              labelPrevious: () => "上个月",
              labelNext: () => "下个月",
            }}
            classNames={{
              root: styles.calendar,
              month_caption: styles.caption,
              caption_label: styles.captionLabel,
              nav: styles.nav,
              button_previous: styles.navButton,
              button_next: styles.navButton,
              month_grid: styles.grid,
              weekday: styles.weekday,
              week: styles.week,
              day: styles.day,
              day_button: styles.dayButton,
              selected: styles.selected,
              today: styles.today,
              outside: styles.outside,
              disabled: styles.disabled,
              hidden: styles.hidden,
            }}
          />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
