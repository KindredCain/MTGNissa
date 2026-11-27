import { NavLink, Outlet, useLocation, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { SearchInput } from "../ui/controls";
import styles from "./layout.module.css";
import { BodyScrollRestoration } from "./BodyScrollRestoration";
import { PageScrollbars } from "../ui/PageScrollbars";
import { CardPreviewScope } from "../cards/CardPreview";
export function Layout() {
  const navigate = useNavigate(),
    location = useLocation();
  function search(value: string) {
    if (!value) return;
    navigate(`/search?q=${encodeURIComponent(value)}`, {
      state: { scroll: "top" },
    });
  }
  const app = useQuery({
    queryKey: ["app"],
    queryFn: ({ signal }) => api.get("/api/v1/app", undefined, { signal }),
    staleTime: Infinity,
  });
  const cards =
    !location.pathname.startsWith("/decks") &&
    !location.pathname.startsWith("/operation_logs");
  return (
    <>
      <BodyScrollRestoration />
      <PageScrollbars />
      <header className={styles.nav} data-preview-header>
        <NavLink to="/sets" className={styles.logo}>
          <img src="/assets/kQcrD.svg" alt="" width="24" height="24" /> MTG
          NISSA
        </NavLink>
        <nav aria-label="主导航">
          <NavLink to="/sets" className={cards ? styles.active : ""}>
            卡牌中心
          </NavLink>
          <NavLink to="/decks">卡组</NavLink>
          <NavLink to="/operation_logs">操作日志</NavLink>
        </nav>
        <div className={styles.right}>
          <SearchInput
            value={
              location.pathname === "/search"
                ? (new URLSearchParams(location.search).get("q") ?? "")
                : ""
            }
            onSubmit={search}
            placeholder="搜索卡牌或输入语法…"
          />
          <a
            href="https://scryfall.com/docs/syntax"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="查看搜索语法帮助"
            className={styles.help}
          >
            <svg
              width="17"
              height="17"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.7"
              strokeLinecap="round"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="9" />
              <path d="M9.5 9a2.5 2.5 0 0 1 5 0c0 2-2.5 2-2.5 4" />
              <path d="M12 16h.01" />
            </svg>
          </a>
          <span>{app.data?.display_name}</span>
        </div>
      </header>
      <CardPreviewScope>
        <Outlet />
      </CardPreviewScope>
    </>
  );
}
export function PageHeader({
  title,
  description,
  children,
  tabs = false,
}: {
  title: string;
  description?: string;
  children?: React.ReactNode;
  tabs?: boolean;
}) {
  const location = useLocation();
  const all_cards =
    location.pathname.startsWith("/sets") || location.pathname === "/search";
  return (
    <section className={styles.pageHeader} data-preview-header>
      <div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      <div>
        {tabs ? (
          <nav aria-label="卡牌中心">
            <NavLink
              to="/sets"
              className={all_cards ? styles.tabActive : undefined}
            >
              全部卡牌
            </NavLink>
            <NavLink to="/collection">我的卡牌</NavLink>
            <NavLink to="/wishlist">愿望单</NavLink>
          </nav>
        ) : (
          children
        )}
      </div>
    </section>
  );
}
export const pageStyles = styles;
