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
| 前端托管 | 前端确定后使用 `embed` 嵌入构建产物 |

以下技术选项仍待确定：

- 是否使用 `goqu` 或其他动态 SQL Builder
- 前端框架
- OpenAPI 生成方案
- 业务模块划分
- API 设计

当前代码只包含数据库连接、卡牌数据初始化和 App DB 迁移，因此直接使用 `database/sql`。开始实现收藏、愿望单、卡组和查询等业务数据访问时，统一引入 `sqlx`，使用其结构体映射、命名参数和查询辅助能力；动态筛选是否再配合 SQL Builder，按实际复杂度决定。

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
- 暂不实现认证

## 延期事项

- 业务模块边界
- SQL 管理和动态查询方案
- API 设计
- 前端技术选型
- 认证扩展
- 多用户或多实例策略
