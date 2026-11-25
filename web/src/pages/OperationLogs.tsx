import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type DTO } from "../api/client";
import {
  action_names,
  finish_names,
  language_name,
} from "../shared/display-names";
import { PageHeader, pageStyles } from "../app/Layout";
import { Modal, validation_error } from "../ui/feedback";
import { Pagination, Skeleton, Toolbar, useURLState } from "../ui/controls";
import styles from "./logs.module.css";
import { Icon, type IconName } from "../ui/Icon";
import { DateInput } from "../ui/DateInput";
import { parse_date } from "../ui/date";
const action_icons = {
  add_card: { icon: "plus", color: "#247650" },
  remove_card: { icon: "minus", color: "#a64a43" },
  add_wishlist: { icon: "star", color: "#b57816" },
  remove_wishlist: { icon: "star-off", color: "#8a6a32" },
  create_deck: { icon: "folder-plus", color: "#356c82" },
  delete_deck: { icon: "folder-minus", color: "#a64a43" },
  copy_deck: { icon: "copy", color: "#5e6f9a" },
  modify_deck: { icon: "pencil", color: "#5a7166" },
} satisfies Record<
  DTO<"OperationSummaryResponse">["action_type"],
  { icon: IconName; color: string }
>;
function ActionIcon({
  action,
}: {
  action: DTO<"OperationSummaryResponse">["action_type"];
}) {
  const { icon, color } = action_icons[action];
  return (
    <span
      className={styles.actionIcon}
      style={{ color, backgroundColor: `${color}18` }}
    >
      <Icon name={icon} size={13} />
    </span>
  );
}
function date_label(date: string, weekday = true) {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
    weekday: weekday ? "long" : undefined,
    timeZone: "UTC",
  }).format(new Date(`${date}T12:00:00Z`));
}
export function Component() {
  const { params, update } = useURLState(),
    client = useQueryClient(),
    [pending, setPending] = useState(false),
    [detail, setDetail] = useState<DTO<"OperationSummaryResponse"> | null>(
      null,
    );
  const page = Math.max(1, Number(params.get("page")) || 1),
    query = {
      start_date: params.get("start_date") ?? undefined,
      end_date: params.get("end_date") ?? undefined,
      page,
    };
  const list = useQuery({
    queryKey: ["operation_logs", "summary", query],
    queryFn: ({ signal }) =>
      api.get("/api/v1/operation_logs/summary", query, { signal }),
  });
  async function change(
    start_date: string,
    end_date: string,
    next_page: number,
  ) {
    if (pending) return;
    if (start_date && end_date && start_date > end_date) {
      validation_error("开始日期不能晚于结束日期。");
      return;
    }
    setPending(true);
    const next = {
      start_date: start_date || undefined,
      end_date: end_date || undefined,
      page: next_page,
    };
    try {
      await client.fetchQuery({
        queryKey: ["operation_logs", "summary", next],
        queryFn: ({ signal }) =>
          api.get("/api/v1/operation_logs/summary", next, { signal }),
      });
      update({ start_date, end_date, page: String(next_page) });
    } catch {
      /* QueryCache reports the failure and the current page stays visible. */
    } finally {
      setPending(false);
    }
  }
  return (
    <>
      <PageHeader
        title="操作日志"
        description="查看收藏、愿望单与卡组的变更记录"
      />
      <main className={`${pageStyles.content} ${styles.page}`}>
        <Toolbar count={`共 ${list.data?.total_items ?? 0} 条汇总`}>
          <DateFilter
            key={JSON.stringify([query.start_date, query.end_date])}
            start_date={query.start_date ?? ""}
            end_date={query.end_date ?? ""}
            pending={pending}
            onSubmit={(start, end) => void change(start, end, 1)}
          />
        </Toolbar>
        <div className={styles.tableFrame}>
          <table className={`${styles.table} ${styles.summaryTable}`}>
            <thead>
              <tr>
                <th>日期</th>
                <th>操作类型</th>
                <th className={styles.number}>操作次数</th>
                <th className={styles.detailCell} aria-label="明细" />
              </tr>
            </thead>
            <tbody>
              {list.data?.items.map((item) => (
                <tr
                  key={`${item.operation_date}:${item.action_type}`}
                  onClick={() => setDetail(item)}
                >
                  <td className={styles.date}>
                    {date_label(item.operation_date, false)}
                    <span className={styles.weekday}>
                      {new Intl.DateTimeFormat("zh-CN", {
                        weekday: "long",
                        timeZone: "UTC",
                      }).format(new Date(`${item.operation_date}T12:00:00Z`))}
                    </span>
                  </td>
                  <td>
                    <button className={styles.typeLink}>
                      <ActionIcon action={item.action_type} />
                      {action_names[item.action_type]}
                    </button>
                  </td>
                  <td className={`${styles.number} ${styles.count}`}>
                    {item.count} 条
                  </td>
                  <td className={styles.detailCell}>
                    <span className={styles.detailLink} aria-hidden="true">
                      <Icon name="chevron-right" size={15} />
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {list.isPending && <Skeleton />}
        <Pagination
          summary
          page={page}
          pages={list.data?.total_pages ?? 0}
          total={list.data?.total_items ?? 0}
          disabled={pending}
          onChange={(value) => {
            void change(query.start_date ?? "", query.end_date ?? "", value);
          }}
        />
        {detail && <Detail summary={detail} close={() => setDetail(null)} />}
      </main>
    </>
  );
}
function DateFilter({
  start_date,
  end_date,
  pending,
  onSubmit,
}: {
  start_date: string;
  end_date: string;
  pending: boolean;
  onSubmit: (start: string, end: string) => void;
}) {
  const [start, setStart] = useState(start_date),
    [end, setEnd] = useState(end_date);
  return (
    <form
      noValidate
      className={styles.dates}
      onSubmit={(event) => {
        event.preventDefault();
        if ([start, end].some((value) => value !== "" && !parse_date(value))) {
          validation_error("请输入完整有效的日期。");
          return;
        }
        onSubmit(start, end);
      }}
    >
      <DateInput
        label="开始日期"
        value={start}
        onChange={setStart}
        range={{ start, end, target: "start" }}
      />
      <span className={styles.dateSeparator} aria-hidden="true">
        —
      </span>
      <DateInput
        label="结束日期"
        value={end}
        onChange={setEnd}
        range={{ start, end, target: "end" }}
      />
      <button disabled={pending}>查询</button>
    </form>
  );
}
function Detail({
  summary,
  close,
}: {
  summary: DTO<"OperationSummaryResponse">;
  close: () => void;
}) {
  const client = useQueryClient(),
    [page, setPage] = useState(1),
    [pending, setPending] = useState(false);
  const query = {
    operation_date: summary.operation_date,
    action_type: summary.action_type,
    page,
  };
  const list = useQuery({
    queryKey: ["operation_logs", "detail", query],
    queryFn: ({ signal }) =>
      api.get("/api/v1/operation_logs", query, { signal }),
  });
  const app = useQuery({
    queryKey: ["app"],
    queryFn: ({ signal }) => api.get("/api/v1/app", undefined, { signal }),
    staleTime: Infinity,
  });
  async function change(next_page: number) {
    if (pending) return;
    setPending(true);
    const next = { ...query, page: next_page };
    try {
      await client.fetchQuery({
        queryKey: ["operation_logs", "detail", next],
        queryFn: ({ signal }) =>
          api.get("/api/v1/operation_logs", next, { signal }),
      });
      setPage(next_page);
    } catch {
      /* Keep the previous page. */
    } finally {
      setPending(false);
    }
  }
  const card_log = [
      "add_card",
      "remove_card",
      "add_wishlist",
      "remove_wishlist",
    ].includes(summary.action_type),
    quantity_log = ["add_card", "remove_card"].includes(summary.action_type);
  return (
    <Modal
      open
      title={`${date_label(summary.operation_date, false)} · ${action_names[summary.action_type]}`}
      title_meta={`共 ${list.data?.total_items ?? 0} 条操作`}
      className={styles.detail}
      close={pending || list.isPending ? undefined : close}
      close_label="×"
    >
      <div className={styles.tableArea}>
        <table className={`${styles.table} ${styles.detailTable}`}>
          <colgroup>
            <col className={styles.timeColumn} />
            <col />
            {card_log && <col className={styles.printingColumn} />}
            {quantity_log && <col className={styles.quantityColumn} />}
          </colgroup>
          <thead>
            <tr>
              <th>操作时间</th>
              <th>{card_log ? "卡牌" : "卡组名称"}</th>
              {card_log && <th>印刷信息</th>}
              {quantity_log && <th className={styles.number}>数量变化</th>}
            </tr>
          </thead>
          <tbody>
            {list.data?.items.map((item) => (
              <tr key={item.operation_id}>
                <td className={styles.time}>
                  {new Intl.DateTimeFormat("zh-CN", {
                    timeZone: app.data?.timezone ?? "UTC",
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                    hour12: false,
                  }).format(new Date(item.occurred_at))}
                </td>
                <td
                  className={styles.name}
                  title={item.kind === "card" ? item.card.name : item.deck.name}
                >
                  {item.kind === "card" ? item.card.name : item.deck.name}
                </td>
                {item.kind === "card" && (
                  <td className={styles.printing}>
                    {item.card.set_code} · {item.card.collector_number} ·{" "}
                    {language_name(item.card.language)}
                    {quantity_log && item.card.finish !== null
                      ? ` · ${finish_names[item.card.finish]}`
                      : ""}
                  </td>
                )}
                {quantity_log && item.kind === "card" && (
                  <td
                    className={`${styles.number} ${styles.quantity} ${
                      (item.card.quantity_delta ?? 0) > 0
                        ? styles.positive
                        : styles.negative
                    }`}
                  >
                    {(item.card?.quantity_delta ?? 0) > 0 ? "+" : ""}
                    {item.card?.quantity_delta}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {list.isPending && <Skeleton />}
      </div>
      <Pagination
        page={page}
        pages={list.data?.total_pages ?? 0}
        total={list.data?.total_items ?? 0}
        disabled={pending}
        onChange={(value) => {
          void change(value);
        }}
      />
    </Modal>
  );
}
