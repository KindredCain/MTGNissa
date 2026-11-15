import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type DTO } from "../api/client";
import { PageHeader, pageStyles } from "../app/Layout";
import { Modal, validation_error } from "../ui/feedback";
import { Pagination, Skeleton, Toolbar, useURLState } from "../ui/controls";
import styles from "./logs.module.css";
const action_names: Record<DTO<"ActionType">, string> = {
  add_card: "增加卡牌",
  remove_card: "减少卡牌",
  add_wishlist: "加入愿望单",
  remove_wishlist: "移出愿望单",
  create_deck: "创建卡组",
  delete_deck: "删除卡组",
  copy_deck: "复制卡组",
  modify_deck: "修改卡组",
};
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
      <PageHeader title="操作日志" description="查看卡牌与卡组的操作记录" />
      <main className={pageStyles.content}>
        <Toolbar count={`共 ${list.data?.total_items ?? 0} 条汇总`}>
          <DateFilter
            key={JSON.stringify([query.start_date, query.end_date])}
            start_date={query.start_date ?? ""}
            end_date={query.end_date ?? ""}
            pending={pending}
            onSubmit={(start, end) => void change(start, end, 1)}
          />
        </Toolbar>
        <table>
          <thead>
            <tr>
              <th>日期</th>
              <th>操作类型</th>
              <th>操作次数</th>
            </tr>
          </thead>
          <tbody>
            {list.data?.items.map((item) => (
              <tr key={`${item.operation_date}:${item.action_type}`}>
                <td>{date_label(item.operation_date)}</td>
                <td>
                  <button
                    className={styles.typeLink}
                    onClick={() => setDetail(item)}
                  >
                    {action_names[item.action_type]}
                  </button>
                </td>
                <td>{item.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {list.isPending && <Skeleton />}
        <Pagination
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
      className={styles.dates}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(start, end);
      }}
    >
      <label>
        开始日期
        <input
          type="date"
          value={start}
          onChange={(event) => setStart(event.target.value)}
        />
      </label>
      <label>
        结束日期
        <input
          type="date"
          value={end}
          onChange={(event) => setEnd(event.target.value)}
        />
      </label>
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
        <table>
          <thead>
            <tr>
              <th>操作时间</th>
              <th>{card_log ? "卡牌" : "卡组名称"}</th>
              {card_log && <th>印刷信息</th>}
              {quantity_log && <th>数量变化</th>}
            </tr>
          </thead>
          <tbody>
            {list.data?.items.map((item) => (
              <tr key={item.operation_id}>
                <td>
                  {new Intl.DateTimeFormat("zh-CN", {
                    timeZone: app.data?.timezone ?? "UTC",
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                    hour12: false,
                  }).format(new Date(item.occurred_at))}
                </td>
                <td>
                  {item.kind === "card" ? item.card.name : item.deck.name}
                </td>
                {item.kind === "card" && (
                  <td>
                    {item.card.set_code} · #{item.card.collector_number} ·{" "}
                    {item.card.language_display_name}
                    {quantity_log
                      ? ` · ${item.card.finish === "foil" ? "闪卡" : "普通"}`
                      : ""}
                  </td>
                )}
                {quantity_log && item.kind === "card" && (
                  <td
                    className={
                      (item.card?.quantity_delta ?? 0) > 0
                        ? styles.positive
                        : styles.negative
                    }
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
