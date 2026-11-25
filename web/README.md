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

## 构建

在 `web` 目录执行：

```powershell
corepack pnpm build
```

构建产物输出到 `dist/`。Vite 构建不执行类型检查，类型检查使用 `corepack pnpm typecheck`。

## 浏览器访问兼容

支持 localhost HTTP、域名 HTTPS、IP 加端口 HTTP、普通域名 HTTP 及 IP 加端口 HTTPS。HTTPS 证书须受浏览器信任并匹配访问的域名或 IP；证书及监听配置在整包部署阶段处理。

UUID 使用 `crypto.getRandomValues` 生成 RFC 4122 v4 格式，不依赖仅安全上下文可用的 `crypto.randomUUID`。复制卡组代码优先使用 Clipboard API；普通 HTTP 下该 API 不可用时，使用用户点击触发的复制命令。浏览器拒绝复制时仍显示统一错误弹窗。

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
| `src/ui/` | 公共控件、Lucide 图标、弹窗和错误反馈 |
| `src/shared/` | 跨页面共用的展示映射和辅助函数 |
| `src/test/` | 单元／组件测试的初始化和测试数据；用例位于对应源码旁 |
| `public/` | 静态资源 |
| `scripts/` | API 类型生成与契约检查脚本 |
| `e2e/` | 浏览器测试与模拟接口 |

具体系列卡牌的本地排序 URL 参数统一使用 `_asc`／`_desc` 后缀，默认 `collector_number_asc`；这些参数不发送给后端卡牌列表接口。

## 页面

枚举展示名集中维护在 `src/shared/display-names.ts`，包含语言、颜色与颜色分组、稀有度、工艺、卡组分区、卡牌类型分组、日志操作、卡组异常及按场景区分的排序名称。语言通过 `language_name(code)` 查询，未收录的代码原样显示；固定枚举直接读取对应名称映射。工艺统一显示“普通／闪卡”，排序统一使用上下箭头，颜色保持固定顺序且不加箭头。该文件只维护名称，不包含选项范围、排列顺序、默认值、请求或排序实现；这些规则保留在业务模块，`type_order` 使用明确数组维护。当前不引入翻译资源、界面语言选择或国际化框架。

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

## 后续移动端计划

移动端不提供卡组编辑。列表中的新建、编辑、复制入口及详情页中的编辑、复制入口保留原有样式，但不响应操作、不发送新建或复制请求；直接访问 `/decks/:deck_id/edit` 时转到对应详情页。此方案为后续版本预留，本期暂不实现设备判断和路由拦截。

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `corepack pnpm typecheck` | TypeScript 类型检查 |
| `corepack pnpm lint` | 代码检查 |
| `corepack pnpm test` | 单元和组件测试 |
| `corepack pnpm test:e2e` | 浏览器测试 |
| `corepack pnpm build` | 构建，输出到 `dist/` |

首次运行浏览器测试前，执行 `corepack pnpm exec playwright install chromium`。
