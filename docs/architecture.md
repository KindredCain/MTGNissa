# MTGNissa 技术架构设计

## 文档范围

- 设计依据：需求基线、原型、新数据字典和 API 契约
- 范围：技术架构、运行边界和部署方式
- 详细业务表结构见个人数据库设计；功能行为见功能需求。本文不重复完整 API 契约和前端交互设计。

个人数据库结构与一致性规则见 [个人数据库设计](app-data-schema.md)，业务接口见 [API 契约](api-contract.md)。本文规定组件职责、数据库边界、运行生命周期和部署约束。

## 系统定位

MTGNissa 采用以下运行模型：

- 单用户
- 单应用实例
- 单个人数据库
- 外部卡牌元数据库，日常业务只读
- 单 Docker 容器
- 首版不提供应用内认证

“单个人数据库”表示一个 MTGNissa 实例只管理一个个人数据库。卡牌数据库是独立的外部只读数据源，不属于个人数据库。

## 总体架构

前端静态产物与 Go 服务部署在同一容器中，运行结构如下：

```text
浏览器
  │
  ▼
MTGNissa Docker 容器
├── 前端静态资源
├── Gin HTTP 服务
├── 应用逻辑
└── 数据访问
      │
      ├── Card DB 连接池
      │     └── 外置卡牌数据库，业务只读、可选维护加载
      │
      └── App DB 连接池
            └── 外置个人数据库，读写
```

容器内部不运行 MySQL、Redis、消息队列、反向代理或其他微服务。

## 技术选型

| 范围 | 选择 |
| --- | --- |
| 后端语言 | Go |
| HTTP 框架 | Gin |
| 数据库 | 外置 MySQL |
| MySQL 驱动 | `go-sql-driver/mysql` |
| 底层连接池 | Go 标准库 `database/sql` |
| 业务 SQL 访问 | `sqlx`，复用底层连接池并承担业务映射、查询与事务 |
| 动态 SQL 组装 | `goqu`，仅用于 Set、全局搜索、收藏和愿望单列表 |
| 数据库迁移 | Goose，仅用于个人数据库 |
| 日志 | 标准库 `slog` |
| 配置 | YAML 文件，支持环境变量覆盖 |
| 依赖管理 | Go Modules |
| 依赖注入 | 构造函数手工注入 |
| 部署 | 单 Docker 镜像 |
| 前端语言 | TypeScript，开启严格模式 |
| 前端框架 | React 客户端单页应用（SPA） |
| 前端构建 | Vite；生产构建产物由 Go `embed` 嵌入 |
| 前端路由 | React Router Data Mode；使用浏览器历史路由、嵌套路由和路由级懒加载 |
| 服务端状态 | TanStack Query；负责请求缓存、失效、mutation 和游标无限加载 |
| 复杂编辑状态 | 普通页面使用 React 本地状态；卡组未保存编辑态使用 Zustand |
| 前端样式 | CSS Modules、CSS Custom Properties、Grid/Flex、媒体查询和容器查询 |
| 无样式交互组件 | Radix UI Primitives，按需用于弹窗、下拉框、气泡和表单控件 |
| 卡组拖拽 | dnd-kit；首版使用 Pointer Sensor 和 Touch Sensor，Keyboard Sensor 及其他非拖拽换区方式列入后续版本计划 |
| 图表 | Apache ECharts，按页面懒加载并响应容器尺寸变化 |
| 长列表 | 按端点区分游标分页、完整轻量数据本地分批渲染和固定页码；需要窗口化时使用 TanStack Virtual |
| 前端表单 | React Hook Form + Zod；只负责客户端输入与类型校验，不代替后端业务校验 |
| API 类型 | 以 OpenAPI 3.1 契约生成 TypeScript 类型，生成工具使用 `openapi-typescript` |
| 前端测试 | Vitest、React Testing Library、MSW 和 Playwright |
| 前端包管理 | pnpm，通过 Corepack 固定包管理器版本 |
| 前端代码质量 | ESLint、typescript-eslint、Prettier 和独立 TypeScript 类型检查 |

以下技术选项仍待确定：

- OpenAPI 契约的发布、校验和后端同步流程

业务数据访问统一使用 `sqlx`。动态筛选、排序和游标条件使用 `goqu` 构建，结构固定的 SQL、迁移和卡牌导入保持手写，不为形式统一改写为 Builder。

收藏和愿望单使用 App DB 查询副本完成筛选、聚合和分页，展示元数据仍由 Card DB 批量提供；options 先确定涉及的系列、语言代码，再读取 Card DB 元数据。

## 后端分层与事务边界

业务 SQL 统一放在 `internal/store` 包内，按业务分文件和 Store 类型；不再以 HTTP 路径或数据库名称将不同业务的全部 SQL 集中到一个 `AppStore`。卡牌、系列、Token、收藏、愿望单、卡组和日志查询或写入分别归入对应存储实现。`store` 不引用 `cards`、`sets`、`tokens`、`collection` 等 Service 包。

`internal/model` 保存跨层使用的业务模型、查询参数、分页及汇总结果，供 Service、Store 和 HTTP 映射共同使用。HTTP 请求与响应 DTO 继续留在 `internal/httpapi`，通用信封留在 `internal/httpresponse`；`internal/appdata` 的数据库实体和迁移，以及 `internal/carddata` 的导入、解析、转换模型均保留原位。仅在实现内部使用的 SQL 扫描行、分组键和游标编码结构不进入共用业务模型。

依赖方向为 `httpapi -> 业务 Service -> store`，Service 和 Store 都可引用 `model`；`model` 不反向引用 Service 或 Store。业务变更、版本选择、详情组织和跨业务编排由 Service 负责；Store 负责 SQL 查询、扫描与基础映射、锁定及持久化，也可封装系列语言和层级等查询结果整理。跨层传递的业务数据放在 `model`。分层调整应保留合并查询、批量处理和一致性快照，避免增加数据库往返、全量数据累计或重复计算。Service 按现有 API 模块组织流程，`/cards/{printing_uuid}/collection_adjustments` 和 `/cards/{printing_uuid}/wishlist_state` 继续由 `cards.Service` 编排，分别调用收藏和愿望单存储能力。

Service 决定事务包含哪些业务步骤，`store.AppTransactions` 封装开启、提交与回滚。写入通过 `WithAppTx` 回调取得绑定到同一个 `sqlx.Tx` 的 `AppTx`，各业务存储不自行提交，也不在事务中改用普通 DB 连接。业务变更及其操作日志由 Service 在同一回调中组合，任何一步失败均回滚。需要串行更新同一资源时，在事务中锁定对应记录，再读取和修改其关联数据；规范化后的业务状态未变化时，不更新数据、更新时间或操作日志。

需要幂等保护的操作共用有界进程内缓存，只保存请求指纹、执行状态、过期时间及必要的资源标识，不缓存完整响应。事务成功提交后，才允许将记录标记为完成；重复请求重新查询对应资源的当前权威状态，提交后的响应查询失败不得导致重试重复写入。

`WithAppRead` 提供只读、可重复读快照，用于组合详情中的收藏、愿望单及卡组占用，或查询调整后的权威汇总。Card DB 身份查询在开启 App DB 写事务前完成；两个数据源保持独立连接，不提供跨库原子事务或全局一致快照。数据导入、数据库迁移和健康检查保持原有基础设施边界。

## 前端架构

前端采用 TypeScript、React 和 Vite 实现客户端单页应用。系统面向个人部署，不需要搜索引擎收录或首屏服务端渲染，因此不引入 Next.js、React Server Components 或其他 SSR 运行时。React Router 使用 Data Mode 组织嵌套路由、路由级错误边界和代码拆分；业务数据请求、缓存和 mutation 统一由 TanStack Query 管理，避免在路由状态、组件状态和自建全局缓存中重复保存服务端数据。

前端状态按职责划分：

- URL 保存当前路由、搜索关键字、已提交筛选、排序和详情标识，使页面可以刷新、收藏和直接访问。
- TanStack Query 保存 Set、卡牌、收藏、愿望单、卡组和操作日志等服务端状态及游标分页结果。
- React 组件本地状态保存下拉框临时选择、弹窗开关、正反面切换和气泡状态。
- Zustand 仅用于卡组编辑页的未保存内容、分区和数量、`dirty` 状态及离开确认，不作为通用服务端缓存。

前端源代码统一放置在根目录 `web`。开发环境由 Vite Dev Server 提供页面，并把 `/api` 和 `/health` 代理到 Go 服务；生产环境执行 Vite 构建，由 Go `embed` 嵌入静态产物并与 API 同源提供。Go 服务必须为非 `/api`、非 `/health` 且未命中静态文件的浏览器路由返回前端 `index.html`，保证详情页等深层路由可以直接访问和刷新。生产部署不依赖独立 Node.js 进程。

基础交互组件使用 Radix UI Primitives 封装为项目内部组件，视觉样式仍以原型和项目设计令牌为准，不直接引入带有完整视觉体系的组件库。卡组统计图表使用 Apache ECharts，并按页面进行代码拆分；图表通过容器尺寸而不是固定视口尺寸决定宽度。首版卡组分区移动使用 dnd-kit 实现拖拽，不提供按钮、菜单或键盘等非拖拽换区方式；非拖拽换区入口列入后续版本计划。

### 查询与列表状态

- Set 索引查询、筛选、排序和游标只作用于顶层父组，完整子树始终跟随并缩进；同父节点先普通子 Set、后 Token Set，各组内 `released_at ASC, code ASC`，递归前序展开，分页不拆分子树。
- 普通及 Token Set 卡牌一次取得完整轻量数据，图片和 DOM 在前端本地分批渲染；普通 Set 的筛选与排序复用完整结果。全局搜索、收藏、愿望单、卡组列表和增加卡牌正式结果使用服务端游标；操作日志固定每页 10 条。
- 普通卡牌与 Token 列表分别使用 `/api/v1/cards` 和 `/api/v1/tokens`，均通过必填查询参数 `set_code` 指定系列；Token 入口仅接受 `set_code`，不接受语言、名称搜索、筛选、排序或分页参数。
- 卡组增加卡牌把正在输入的关键字、正式提交的关键字和联想结果分开保存。正式 `printed_name LIKE '%q%'` 只由按钮或联想点击触发，按钮无最小长度，空关键字表示不限制名称；自动 `q%` 联想满足至少 3 个英文字母或 2 个汉字后防抖请求，最多 5 个去重名称，丢弃过期响应，不刷新正式结果。
- 语言名称直接消费字典 `display_name`；普通 Set 索引固定按字典顺序取前 11 个语言，详情版本列表不套用数量限制，Token 不指定语言。Token 固定无搜索、筛选、排序与普通卡详情入口。
- 收藏和愿望单 mutation 成功后才更新显示，等待期间禁用当前卡位相关操作。收藏两个合并开关任一开启时只显示汇总数量，不能使用代表卡 UUID 调整数量。
- 收藏调整、愿望单状态、创建与复制卡组按 API 契约使用进程内、有界、30 分钟有效期的幂等记录，不在 App DB 建幂等表；保存、只读查询及卡牌加载不套用该要求。

### 请求与缓存边界

- 页面滚动统一管理：正常进入新页面及全局搜索时置顶，浏览器前进／后退恢复对应历史记录的位置；卡牌语言／版本切换保持位置，筛选等页面内操作沿用各自规则。历史恢复等待异步内容加载，并在用户主动操作时停止自动调整。

- 进入新页面（包括全局搜索）先导航，由目标页加载数据；当前页面的服务端筛选、分页及卡牌语言／版本切换，先取得数据成功再更新显示和地址。普通 Set 的本地筛选、排序及编辑页本地操作不发请求，也不因筛选排序重新挂载取数。
- 写入成功后更新或失效相关缓存，再进入目标页加载；复制进入新卡组编辑页，删除清理详情并返回列表。所有会写入操作日志的业务操作同时失效日志缓存。请求中的旧结果不得覆盖后续操作或新编辑会话。
- 查询默认使用 30 秒新鲜期。每次进入卡组编辑页重新读取已保存状态，当前会话中的未保存内容不受后台重新取数影响，关闭或放弃后丢弃。卡牌编辑上下文缓存不因闲置自动回收，退出会话时主动清理。
- 请求失败保留当前操作状态；再次提交同一写入意图按契约重试，关闭错误弹窗不自动重试。
- 固定枚举的界面文案由前端统一映射；名称、规则文字、语言展示名及历史快照等服务端展示数据直接使用响应值。

### 本期界面范围

本期仅交付桌面页面及桌面测试：1440 px 显示五列卡图，1366 px 和 1280 px 显示四列，低于 1280 px 保留整页横向滚动，不整页缩放。移动端页面、触屏专项交互及卡组编辑入口拦截均属后续版本，计划见 [前端说明](../web/README.md)。

### 移动兼容开发约束

移动兼容约束持续适用，用于保留后续适配能力，不代表本期交付移动端页面或专项测试。

- 数据请求、业务状态和展示布局分离；后续移动端复用现有业务能力，通过媒体查询或容器查询适配布局，不使用 User-Agent 决定布局。
- 使用流式布局和设计令牌，不依赖整页绝对坐标或等比缩放；卡图保持比例及加载占位，列表按需加载图片。
- 通用指针交互优先使用 Pointer Events；悬停信息保留点击、焦点或详情等访问途径，触屏专项交互在后续移动端开发时完善。
- 弹窗、抽屉、气泡和图表内容与桌面定位分离，保留视口边界与滚动能力，便于后续调整移动端呈现。
- 移动端不提供卡组编辑，不要求适配编辑页布局、拖拽或数量编辑；相关入口保留原样但不可交互，直接访问编辑路由时转到卡组详情。该限制随后续移动端版本实现。

## 数据库边界

### 卡牌数据库

- 外部部署
- 使用独立连接配置
- 日常卡牌查询只读
- 不使用 Goose，不与个人数据库迁移混用
- 卡牌数据加载默认关闭；显式启用后，维护接口可以全量替换固定白名单表
- 关闭加载功能时使用只读账号；启用时账号需要白名单表所需的 DDL 和 DML 权限

普通具体语言版本查询使用 `mtgnissa_card_printing -> mtgnissa_card -> 翻译或 Oracle 数据`，通过 `card_uuid` 关联基础卡面；业务入口为 `printing_uuid = mtgnissa_card_printing.uuid`，多牌面使用同一印刷、同一语言主牌面的 UUID，完整牌面随同返回。基础卡面 `uuid` 用于卡面与翻译关联，`oracle_id` 用于逻辑卡身份。卡面 Oracle 翻译经 `card_oracle_translation_mapping` 关联，不能假设基础卡表有 `face_oracle_id`。

全局搜索命中、卡组代码解析、逻辑卡聚合与统计及 Oracle 查询可直接使用 `mtgnissa_card`。全局结果按 Oracle 去重，代表基础印刷取最小 `scryfall_id` 后取主牌面，仅为详情入口解析对应语言印刷 UUID。Token 直接使用 `mtgnissa_token`，不进入普通卡语言、详情、收藏、愿望单、卡组或全局搜索链路。系列以 `mtgnissa_set.code` 为业务标识，层级唯一依据 `parent_code`。

图片由后端解析为 `CardImageDTO`，前端不拼接地址；语言印刷缺少图片 ID 时仅图片来源回退基础卡图，不能改变业务 UUID、语言、分组、路由或个人状态。历史日志直接读取文字及身份快照，不以当前 Card DB 重新补齐或改写，也不保存或返回卡图。

卡牌加载按新字典读取九个文件：AllPrintings 标准 JSON、七个旧 NDJSON 及 Oracle Tags 标准 JSONL；旧卡文件只作翻译 UUID 桥，废弃 `rulings.jsonl` 不属于输入。加载使用固定白名单临时表和原子换表，不写 App DB，不依赖 Redis 或外部搜索引擎。

### 个人数据库

- 外部部署
- 一个实例对应一个个人数据库
- 应用拥有读写权限
- 由 Goose 管理结构迁移
- 保存当前实例所有者的数据

个人数据库的表结构和一致性规则见 [个人数据库设计](app-data-schema.md)。系统采用单实例单用户模型，不创建 `users` 或 `app_profile` 表，业务表不保存 `user_id`；页面可通过配置文件中的可选 `app.display_name` 标识部署实例。

## 连接池设计

应用启动时为两个数据库各创建一个长期存在的 `sqlx.DB` 权威句柄：

```go
type Databases struct {
	Card *sqlx.DB
	App  *sqlx.DB
}
```

`sqlx.DB` 包装并复用底层的 `database/sql` 连接池，不表示一条固定的 MySQL 连接，也不另外创建连接池。业务 Store 接收对应的 `*sqlx.DB`；迁移、卡牌数据导入和健康检查只接收其内嵌的 `*sql.DB`，这些底层模块不依赖 `sqlx`。连接由底层连接池按需创建、复用和回收，并由唯一的 `sqlx.DB` 权威句柄统一关闭。

个人部署的初始建议值：

```text
每个连接池：
- MaxOpenConns: 5
- MaxIdleConns: 1
- ConnMaxIdleTime: 5m
- ConnMaxLifetime: 30m
```

这些参数必须配置化。单次请求不创建或关闭连接池，应用退出时统一关闭两个连接池。

## 依赖组织

Gin 不承担依赖注入。应用使用构造函数显式组装依赖：

```text
配置
  ├──► Card DB
  ├──► App DB
  ├──► 数据访问组件
  ├──► 应用逻辑组件
  ├──► HTTP Handler
  └──► Gin Router
```

依赖统一在启动层组装：

```go
type Application struct {
	Router    *gin.Engine
	Databases *database.Databases
	Config    config.Config
	Logger    *slog.Logger
}
```

不使用全局数据库变量、自动扫描、反射式 IoC 或依赖注入容器。

## 目录组织（摘要）

```text
mtgnissa/
├── cmd/
│   └── server/
│       └── main.go
├── docs/
│   ├── requirements.md
│   ├── architecture.md
│   ├── api-contract.md
│   ├── search-syntax-support.md
│   ├── card-data-dictionary.md
│   └── app-data-schema.md
├── design/
│   ├── mtgnissa.pen
│   ├── README.md
│   ├── assets/
│   └── pages/
├── internal/
│   ├── appdata/
│   │   ├── migrations.go
│   │   └── models.go
│   ├── carddata/
│   ├── model/
│   ├── store/
│   ├── cards/
│   ├── collection/
│   ├── decks/
│   ├── operationlogs/
│   ├── sets/
│   ├── search/
│   ├── tokens/
│   ├── wishlist/
│   ├── apperror/
│   ├── bootstrap/
│   ├── config/
│   ├── database/
│   ├── health/
│   ├── httpapi/
│   └── httpresponse/
├── migrations/
│   ├── app/
│   └── embed.go
├── .gitignore
├── config.example.yaml
├── README.md
└── go.mod
```

根目录 `migrations` 统一保存并嵌入 App DB 数据库版本文件；`internal/appdata` 负责执行个人数据库迁移和定义持久化结构对象。Card DB 由卡牌数据加载器管理，不进入 Goose。收藏、愿望单和卡组业务逻辑按业务模块组织，不堆叠在迁移执行代码中。

`internal/httpapi` 作为同一个 HTTP 适配层包按职责和业务能力拆文件：`router.go` 只组装全局中间件、健康检查、API 路由与兜底路由，`middleware.go` 保存通用中间件，`request.go` 保存请求解码辅助，每个业务模块使用 `<module>_handler.go`、`<module>_dto.go` 和对应测试文件。业务路由默认直接在 `router.go` 注册，不以路由数量作为拆分依据；只有模块存在复杂模块级中间件时，才增加 `<module>_routes.go`。接口 DTO 默认保持未导出，不为每个小型业务模块提前创建子包；只有单个模块明显膨胀且能够通过小型公开接口独立时再拆分子包。

## HTTP 基础能力

基础 HTTP 层负责：

- Panic Recovery
- Request ID
- 访问日志
- 请求体大小限制
- `/health/live`
- `/health/ready`

HTTP 响应采用“固定响应信封、业务错误类型、HTTP 层统一映射、边界日志”四层结构。`internal/apperror` 定义与 HTTP 无关的稳定错误码、安全文案、字段或行级详情及仅供日志使用的底层 `Cause`；`internal/httpresponse` 把错误码集中映射为 HTTP 状态并生成统一信封。Handler 和中间件成功时只能调用 `WriteJSON`，失败时只能调用 `WriteAppError`；实际写入错误信封的 `writeError` 保持为包内实现，不允许业务代码直接调用 Gin 的 JSON 输出。数据库模型不得直接作为响应 DTO，底层 SQL、堆栈、文件系统路径和原始异常对象不得进入客户端响应。

请求边界只记录一次错误日志，并携带请求 ID、方法、路径、状态码、错误码、耗时和底层 `Cause`；普通成功请求记录信息日志，可预期的 4xx 拒绝记录警告日志，5xx 与依赖故障记录错误日志。后台任务查询成功但任务执行失败时仍返回 HTTP 200，执行错误保留在 `data.error`，不进入顶层 HTTP `error`。

HTTP 基础层统一处理不存在路由、错误方法、请求体大小、JSON 媒体类型、未知查询参数和 Panic，并统一约束请求处理超时。开发环境通过 Vite 代理访问后端；需要跨源访问时，按部署配置处理 CORS。

HTTP 超时固定在代码中：请求头读取 5 秒、请求读取 30 秒、同步请求执行 30 秒、响应写回 35 秒、空闲连接 60 秒。执行期限通过请求 Context 传递给 SQL 查询与事务，超时后的响应通过统一信封返回 `504 request_timeout`；写回期限多预留 5 秒，用于取消查询及返回错误。执行期限通过 Context 协作取消，不启动并发写响应的超时处理协程。后台卡牌数据加载使用应用生命周期 Context，不因启动请求结束或超时而取消。健康检查内部的两秒依赖超时仍返回 `503 dependency_unavailable`。

`/health/live` 只用于判断进程和 HTTP 服务是否正常。`/health/ready` 使用两秒超时依次检查 Card DB 与 App DB；任一数据库不可用时通过统一错误链路返回 `503 dependency_unavailable`。

## 生命周期

启动顺序：

```text
加载内置默认值、YAML 文件和环境变量覆盖
     ↓
校验配置
     ↓
初始化日志
     ↓
创建 Card DB 连接池
     ↓
创建 App DB 连接池
     ↓
执行数据库连通性检查
     ↓
执行 App DB 迁移
     ↓
执行应用初始化步骤
     ↓
组装应用依赖
     ↓
创建 Gin Router
     ↓
启动 HTTP Server
```

应用初始化阶段按顺序执行具名步骤，记录开始、完成、失败及耗时；新增初始化任务加入该步骤列表。首个步骤更新待同步卡牌映射，单条失败记录后继续，无法读取待处理列表或保存失败文件等步骤级错误会终止启动。

退出顺序：

```text
收到 SIGTERM 或 SIGINT
     ↓
HTTP Server 停止接收新请求
     ↓
等待现有请求结束
     ↓
关闭 Card DB 连接池
     ↓
关闭 App DB 连接池
     ↓
进程退出
```

## 配置

完整 YAML 示例见 [`config.example.yaml`](../config.example.yaml)。配置加载顺序为：内置默认值、YAML 文件、环境变量覆盖。

主要环境变量覆盖项包括：

```text
CONFIG_FILE
HTTP_ADDR
CURSOR_PAGE_SIZE
LOG_LEVEL
LOG_DIR
APP_DISPLAY_NAME
APP_TIMEZONE

CARD_DATA_DIR
CARD_DATA_LOAD_ENABLED

CARD_DB_HOST
CARD_DB_PORT
CARD_DB_NAME
CARD_DB_USER
CARD_DB_PASSWORD

APP_DB_HOST
APP_DB_PORT
APP_DB_NAME
APP_DB_USER
APP_DB_PASSWORD
```

连接池参数分别使用 `CARD_DB_*` 和 `APP_DB_*` 前缀配置。YAML 是常规部署入口，数据库密码等敏感值可以只通过环境变量或 Docker Secret 注入。

数据库密码不得写入 Git 仓库、Dockerfile、Docker 镜像、默认配置文件或日志。

## Docker 部署

最终镜像只包含 Go 可执行文件、前端构建产物以及运行所需的 CA 证书和时区数据。

容器要求：

- 使用非 root 用户运行
- 保持无状态
- 不绑定业务数据卷
- 日志输出到标准输出
- 通过只读 YAML 文件读取配置，并支持环境变量覆盖
- 提供健康检查接口
- 支持优雅退出
- 可以直接删除并重新创建

数据库备份由外部 MySQL 部署体系负责，不由应用容器负责。

## 安全边界

首版不提供登录、Session、JWT、OAuth、用户角色或数据权限过滤。

无认证阶段的服务应部署在可信局域网、VPN、Tailscale 或具有访问控制的反向代理之后，不应直接暴露在未受保护的公网环境中。

HTTP 层应保留未来接入统一认证中间件的能力。

## 已确定事项

- Go + Gin
- 外置双 MySQL 数据源
- Card DB 日常业务只读，维护加载是显式启用的唯一写入入口
- 单实例对应单个人数据库
- 底层使用 `database/sql` 连接池，业务数据访问使用 `sqlx`
- 使用构造函数手工注入
- 仅迁移个人数据库
- 单 Docker 容器部署
- TypeScript + React + Vite 客户端单页应用
- React Router、TanStack Query、CSS Modules 和按需无样式交互组件
- 前端生产构建产物由 Go `embed` 嵌入，不运行独立 Node.js 服务
- 首版按桌面原型交付并保留移动兼容开发约束；移动端专项开发与测试列入后续版本，移动端不提供卡组编辑
- 首版不提供认证

## 延期事项

- OpenAPI 契约的发布、校验和后端同步流程
- 卡组分区移动的按钮、菜单或键盘等非拖拽操作路径
- 平板和手机页面的具体原型、断点验收值及正式交付范围
- 认证扩展
- 多用户或多实例策略
