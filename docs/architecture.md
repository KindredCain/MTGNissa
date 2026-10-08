# MTGNissa 技术架构设计

## 文档状态

- 状态：当前设计
- 范围：技术架构、运行边界和部署方式
- 详细业务表结构见个人数据库设计；功能行为见功能需求。本文不重复完整 API 契约和前端交互设计。

个人数据库业务结构见 [个人数据库设计](app-data-schema.md)。

## 系统定位

MTGNissa 当前采用以下运行模型：

- 单用户
- 单应用实例
- 单个人数据库
- 外部卡牌元数据库，日常业务只读
- 单 Docker 容器
- 暂不实现应用内认证

“单个人数据库”表示一个 MTGNissa 实例只管理一个个人数据库。卡牌数据库是独立的外部只读数据源，不属于个人数据库。

## 总体架构

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
| 业务 SQL 访问 | `sqlx`，在业务数据访问层开始实现时引入 |
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
| 卡组拖拽 | dnd-kit，统一支持 Pointer、Touch 和 Keyboard Sensor |
| 图表 | Apache ECharts，按页面懒加载并响应容器尺寸变化 |
| 长列表 | 优先使用游标分批加载；需要窗口化时使用 TanStack Virtual |
| 前端表单 | React Hook Form + Zod；只负责客户端输入与类型校验，不代替后端业务校验 |
| API 类型 | 以 OpenAPI 3.1 契约生成 TypeScript 类型，生成工具使用 `openapi-typescript` |
| 前端测试 | Vitest、React Testing Library、MSW 和 Playwright |
| 前端包管理 | pnpm，通过 Corepack 固定包管理器版本 |
| 前端代码质量 | ESLint、typescript-eslint、Prettier 和独立 TypeScript 类型检查 |

以下技术选项仍待确定：

- 是否使用 `goqu` 或其他动态 SQL Builder
- OpenAPI 契约的发布、校验和后端同步流程
- 业务模块划分
- API 设计

当前代码只包含数据库连接、卡牌数据初始化和 App DB 迁移，因此直接使用 `database/sql`。开始实现收藏、愿望单、卡组和查询等业务数据访问时，统一引入 `sqlx`，使用其结构体映射、命名参数和查询辅助能力；动态筛选是否再配合 SQL Builder，按实际复杂度决定。

## 前端架构

前端采用 TypeScript、React 和 Vite 实现客户端单页应用。当前系统是个人部署的业务应用，不需要搜索引擎收录或首屏服务端渲染，因此不引入 Next.js、React Server Components 或其他 SSR 运行时。React Router 使用 Data Mode 组织嵌套路由、路由级错误边界和代码拆分；业务数据请求、缓存和 mutation 统一由 TanStack Query 管理，避免在路由状态、组件状态和自建全局缓存中重复保存服务端数据。

前端状态按职责划分：

- URL 保存当前路由、搜索关键字、已提交筛选、排序和详情标识，使页面可以刷新、收藏和直接访问。
- TanStack Query 保存 Set、卡牌、收藏、愿望单、卡组和操作日志等服务端状态及游标分页结果。
- React 组件本地状态保存下拉框临时选择、弹窗开关、正反面切换和气泡状态。
- Zustand 仅用于卡组编辑页的未保存内容、分区和数量、`dirty` 状态及离开确认，不作为通用服务端缓存。

前端源代码统一放置在根目录 `web`。开发环境由 Vite Dev Server 提供页面，并把 `/api` 和 `/health` 代理到 Go 服务；生产环境执行 Vite 构建，由 Go `embed` 嵌入静态产物并与 API 同源提供。Go 服务必须为非 `/api`、非 `/health` 且未命中静态文件的浏览器路由返回前端 `index.html`，保证详情页等深层路由可以直接访问和刷新。生产部署不依赖独立 Node.js 进程。

基础交互组件使用 Radix UI Primitives 封装为项目内部组件，视觉样式仍以原型和项目设计令牌为准，不直接引入带有完整视觉体系的组件库。卡组统计图表使用 Apache ECharts，并按页面进行代码拆分；图表通过容器尺寸而不是固定视口尺寸决定宽度。卡组拖拽使用 dnd-kit，但所有改变分区的业务能力必须同时提供非拖拽操作路径。

### 移动兼容开发约束

首版仍以现有桌面原型和验收规则为准：`1440 px` 视口显示五列卡图，`1366 px` 和 `1280 px` 显示四列，低于 `1280 px` 时继续使用整页横向滚动。在手机原型和对应验收规则确认前，不得自行改变这一既有行为。与此同时，所有新增前端代码必须遵守以下约束，保证后续可以增加平板和手机布局而不重写业务层：

1. 业务状态、数据请求与展示布局必须分离。同一份查询和编辑状态应能够由桌面、平板和手机布局分别渲染，不得在 API 或领域状态中写入视口相关逻辑。
2. 不得使用 User-Agent 判断桌面或手机。布局使用 CSS 媒体查询或容器查询，交互能力使用 Pointer、Touch、Hover 等能力查询或事件模型判断。
3. 指针交互统一优先使用 Pointer Events，不得把核心能力只绑定到鼠标事件。拖拽必须配置触屏激活约束，避免与页面滚动和系统长按手势冲突。
4. 任何通过 Hover 展示的信息都必须同时存在点击、键盘焦点或详情入口；不得存在手机和键盘用户无法访问的只悬停内容。
5. 卡组分区移动、排序等拖拽操作必须提供按钮、菜单或其他非拖拽替代方式。手机支持不得依赖精确拖放才能完成核心任务。
6. 页面布局使用 Grid、Flex、流式容器和设计令牌，不得使用整页 `transform: scale()` 或根据单一设计稿坐标绝对定位整个页面。
7. 卡图组件必须保留固定宽高比和加载占位，并支持延迟加载。无限列表只为已进入当前加载或渲染窗口的卡牌创建图片请求，避免手机端累计大量 DOM 和图片解码任务。
8. 弹窗、抽屉和气泡的业务内容不得依赖固定桌面坐标。桌面可以显示居中弹窗或悬浮层，未来手机布局必须能够复用同一业务组件并切换为全屏页或底部抽屉。
9. 视口固定层使用动态视口单位，并预留 `env(safe-area-inset-*)` 安全区域。主要触摸操作区域不得因桌面视觉压缩为难以点击的小型热区。
10. 卡组详情的八种类型列、宽表格和统计图不得通过缩小文字或点击区域适配手机。后续手机布局应改用分区切换、纵向信息卡或局部横向滚动。
11. 图表和复杂编辑模块必须按路由或功能懒加载；请求需要支持取消或忽略过期结果，筛选快速变化时不得让旧响应覆盖新状态。
12. 自动化测试从首版开始保留桌面和移动设备项目。桌面至少覆盖 `1280 × 720`、`1366 × 768` 和 `1440 × 900`；移动项目在正式手机页面交付前用于发现脚本崩溃、不可关闭弹层和触控事件冲突，不代表手机视觉已经验收。

手机布局启用时，建议以 `768 px` 以下作为手机布局范围、`768 px` 至 `1279 px` 作为平板布局范围，但最终断点必须结合手机和平板原型确认。未来布局应优先采用导航抽屉、筛选抽屉、一至两列卡图、纵向详情、分区式卡组展示以及全屏或底部弹层，不得简单缩小桌面页面。

## 数据库边界

### 卡牌数据库

- 外部部署
- 使用独立连接配置
- 日常卡牌查询只读
- 不使用 Goose，不与个人数据库迁移混用
- 卡牌数据加载默认关闭；显式启用后，维护接口可以全量替换固定白名单表
- 关闭加载功能时使用只读账号；启用时账号需要白名单表所需的 DDL 和 DML 权限

卡牌查询以 `scryfall_card` 为主体。具体印刷及语言版本使用 `scryfall_id`，逻辑卡牌使用 `oracle_id`；牌面级展示和翻译关联继续使用 `uuid`、`face_oracle_id` 与 `face_index`。

### 个人数据库

- 外部部署
- 一个实例对应一个个人数据库
- 应用拥有读写权限
- 由 Goose 管理结构迁移
- 保存当前实例所有者的数据

个人数据库的表结构和一致性规则见 [个人数据库设计](app-data-schema.md)。当前单实例单用户，不创建 `users` 或 `app_profile` 表，业务表不保存 `user_id`；页面可通过配置文件中的可选 `app.display_name` 标识当前部署。

## 连接池设计

应用启动时创建两个长期存在的数据库连接池句柄。业务数据访问层引入后，对外提供 `sqlx.DB`：

```go
type Databases struct {
	Card *sqlx.DB
	App  *sqlx.DB
}
```

`sqlx.DB` 包装并复用底层的 `database/sql` 连接池，不表示一条固定的 MySQL 连接。连接由连接池按需创建、复用和回收。当前初始化阶段的代码仍直接保存 `*sql.DB`；引入业务数据访问层时再切换为 `*sqlx.DB`，不提前增加未使用的依赖。

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

当前阶段不使用全局数据库变量、自动扫描、反射式 IoC 或依赖注入容器。

## 当前目录结构（摘要）

```text
mtgnissa/
├── cmd/
│   └── server/
│       └── main.go
├── docs/
│   ├── requirements.md
│   ├── architecture.md
│   ├── card-data-dictionary.md
│   └── app-data-schema.md
├── internal/
│   ├── appdata/
│   │   ├── migrations.go
│   │   └── models.go
│   ├── bootstrap/
│   ├── config/
│   ├── database/
│   ├── health/
│   └── httpapi/
├── migrations/
│   ├── app/
│   └── embed.go
├── .gitignore
├── config.example.yaml
├── README.md
└── go.mod
```

根目录 `migrations` 统一保存并嵌入数据库版本文件，当前仅包含 App DB 迁移；`internal/appdata` 负责执行个人数据库迁移和定义持久化结构对象。Card DB 仍由卡牌数据加载器管理，不进入 Goose。后续收藏、愿望单和卡组业务逻辑按模块继续拆分，不堆叠在迁移执行代码中。

## HTTP 基础能力

基础 HTTP 层当前提供：

- Panic Recovery
- Request ID
- 访问日志
- 请求体大小限制
- `/health/live`
- `/health/ready`

业务接口继续使用统一 JSON 错误格式。请求超时和开发阶段需要的 CORS 尚未实现，在实际接入前端时补充。

`/health/live` 只用于判断进程和 HTTP 服务是否正常。`/health/ready` 用于判断初始化是否完成；数据库检查策略在实现部署流程时确定。

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
组装应用依赖
     ↓
创建 Gin Router
     ↓
启动 HTTP Server
```

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
SHUTDOWN_TIMEOUT
LOG_LEVEL
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

当前不实现登录、Session、JWT、OAuth、用户角色或数据权限过滤。

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
- 当前按桌面原型交付，新增前端代码必须遵守移动兼容开发约束
- 暂不实现认证

## 延期事项

- 业务模块边界
- SQL 管理和动态查询方案
- API 设计
- OpenAPI 契约的发布、校验和后端同步流程
- 平板和手机页面的具体原型、断点验收值及正式交付范围
- 认证扩展
- 多用户或多实例策略
