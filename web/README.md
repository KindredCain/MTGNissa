# MTGNissa 前端

## 启动

使用 Node.js 24 和 Corepack，pnpm 版本由 `package.json` 固定。

```powershell
cd web
corepack pnpm install --frozen-lockfile
corepack pnpm dev
```

访问 `http://127.0.0.1:5173`。需另行启动 Go 后端，默认代理到 `http://127.0.0.1:8080`。

后端地址不同时，在启动前设置：

```powershell
$env:API_PROXY_TARGET = 'http://127.0.0.1:8081'
```

## API 类型

[OpenAPI](../docs/openapi.json) 是类型生成的唯一输入。契约更新后运行：

```powershell
corepack pnpm generate:api
corepack pnpm check:contract
```

生成文件为 `src/api/schema.d.ts`，不手工修改。生成脚本不会覆盖 OpenAPI。

## 目录

| 目录 | 内容 |
| --- | --- |
| `src/pages/` | 页面入口，路由对应关系见下表 |
| `src/app/` | 全局布局与路由配置 |
| `src/api/` | API 请求、列表查询与生成类型 |
| `src/cards/` | 卡牌展示、筛选、收藏和愿望单操作 |
| `src/decks/` | 卡组编辑、拖拽、统计与编辑状态 |
| `src/ui/` | 公共控件、弹窗和错误反馈 |
| `src/test/` | 单元／组件测试的初始化和测试数据；用例位于对应源码旁 |
| `public/` | 静态资源 |
| `scripts/` | API 类型生成与契约检查脚本 |
| `e2e/` | 浏览器测试与模拟接口 |

## 页面

路由定义在 `src/app/router.tsx`，以下文件均位于 `src/pages/`。

| 页面 | 路由 | 文件 |
| --- | --- | --- |
| 系列列表 | `/sets` | `Sets.tsx` |
| 普通系列卡牌 | `/sets/:set_code/cards` | `SetCards.tsx` |
| Token 系列卡牌 | `/sets/:set_code/tokens` | `SetCards.tsx` 的 `TokenComponent` |
| 全局搜索 | `/search` | `Search.tsx` |
| 卡牌详情 | `/cards/:printing_uuid` | `CardDetail.tsx` |
| 我的卡牌 | `/collection` | `Personal.tsx` |
| 愿望单 | `/wishlist` | `Personal.tsx` 的 `WishlistComponent` |
| 卡组列表 | `/decks` | `DeckList.tsx` |
| 卡组详情 | `/decks/:deck_id` | `DeckDetail.tsx` |
| 卡组编辑 | `/decks/:deck_id/edit` | `DeckEdit.tsx` |
| 操作日志 | `/operation_logs` | `OperationLogs.tsx` |

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `corepack pnpm typecheck` | TypeScript 类型检查 |
| `corepack pnpm lint` | 代码检查 |
| `corepack pnpm test` | 单元和组件测试 |
| `corepack pnpm test:e2e` | 浏览器测试 |
| `corepack pnpm build` | 构建，输出到 `dist/` |

首次运行浏览器测试前，执行 `corepack pnpm exec playwright install chromium webkit`。
