import {
  createBrowserRouter,
  Navigate,
  useRouteError,
  Link,
} from "react-router";
import { Layout } from "./Layout";
function RouteError() {
  const error = useRouteError();
  return (
    <main style={{ padding: 40 }}>
      <h1>页面暂时无法显示</h1>
      <p>
        {error instanceof Error
          ? "请检查网络连接后重新打开页面。"
          : "未找到页面。"}
      </p>
      <Link to="/sets">返回全部卡牌</Link>
    </main>
  );
}
export const router = createBrowserRouter([
  {
    Component: Layout,
    ErrorBoundary: RouteError,
    children: [
      { index: true, element: <Navigate to="/sets" replace /> },
      { path: "sets", lazy: () => import("../pages/Sets") },
      { path: "sets/:set_code/cards", lazy: () => import("../pages/SetCards") },
      {
        path: "sets/:set_code/tokens",
        lazy: async () => ({
          Component: (await import("../pages/SetCards")).TokenComponent,
        }),
      },
      { path: "search", lazy: () => import("../pages/Search") },
      {
        path: "cards/:printing_uuid",
        lazy: () => import("../pages/CardDetail"),
      },
      { path: "collection", lazy: () => import("../pages/Personal") },
      {
        path: "wishlist",
        lazy: async () => ({
          Component: (await import("../pages/Personal")).WishlistComponent,
        }),
      },
      { path: "decks", lazy: () => import("../pages/DeckList") },
      { path: "decks/:deck_id", lazy: () => import("../pages/DeckDetail") },
      { path: "decks/:deck_id/edit", lazy: () => import("../pages/DeckEdit") },
      { path: "operation_logs", lazy: () => import("../pages/OperationLogs") },
      { path: "*", Component: RouteError },
    ],
  },
]);
