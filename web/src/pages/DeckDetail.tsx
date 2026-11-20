import { lazy, Suspense, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { api } from "../api/client";
import { Board } from "../decks/Board";
import { DeleteConfirmation, useDeckActions } from "../decks/actions";
import { Skeleton } from "../ui/controls";
import styles from "../decks/decks.module.css";
import { Bookmark } from "../decks/Bookmark";
import { relative_time } from "../decks/time";
import { Icon } from "../ui/Icon";
const Statistics = lazy(() => import("../decks/Statistics"));
export function Component() {
  const { deck_id = "" } = useParams(),
    [stats, setStats] = useState(false),
    statistics_trigger = useRef<HTMLButtonElement>(null),
    [deleting, setDeleting] = useState(false),
    action = useDeckActions(deck_id);
  const query = useQuery({
      queryKey: ["decks", "detail", deck_id],
      queryFn: ({ signal }) =>
        api.get("/api/v1/decks/{deck_id}", undefined, {
          path: { deck_id },
          signal,
        }),
    }),
    deck = query.data;
  return (
    <main className={styles.dark}>
      {deck ? (
        <>
          <header className={styles.header}>
            <div className={styles.detailIdentity}>
              <h1 title={deck.name}>{deck.name}</h1>
              <p>
                共 {deck.summary.counted_total} 张 · 更新于{" "}
                {relative_time(deck.updated_at)} · 收集率{" "}
                {Math.round(deck.summary.collection_rate * 100)}%
              </p>
            </div>
            <div className={styles.headerActions}>
              <Link to={`/decks/${encodeURIComponent(deck_id)}/edit`}>
                <Icon name="file-text" />
                编辑卡组
              </Link>
              <button
                disabled={action.isPending}
                onClick={() => action.mutate("copy")}
              >
                <Icon name="copy" />
                复制卡组
              </button>
              <button
                disabled={action.isPending}
                onClick={() => action.mutate("code")}
              >
                <Icon name="clipboard-copy" />
                复制卡组代码
              </button>
              <button
                className={styles.danger}
                disabled={action.isPending}
                onClick={() => setDeleting(true)}
              >
                <Icon name="trash-2" />
                删除卡组
              </button>
            </div>
          </header>
          <Board deck={deck} />
          <Bookmark
            top={310}
            hidden={stats}
            button_ref={statistics_trigger}
            onClick={() => setStats(true)}
          />
          {stats && (
            <Suspense>
              <Statistics
                deck={deck}
                close={() => setStats(false)}
                bookmark_top={310}
                trigger_ref={statistics_trigger}
              />
            </Suspense>
          )}
          <DeleteConfirmation
            open={deleting}
            pending={action.isPending}
            cancel={() => setDeleting(false)}
            confirm={() =>
              action.mutate("delete", { onSettled: () => setDeleting(false) })
            }
          />
        </>
      ) : (
        query.isPending && <Skeleton />
      )}
    </main>
  );
}
