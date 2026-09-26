# MTGNissa 技术架构设计

## 文档状态

- 状态：初始设计
- 范围：技术架构、运行边界和部署方式
- 暂不包含：业务模块、业务数据表、API 和前端交互设计

个人数据库业务结构见 [个人数据库设计](app-data-schema.md)。

## 系统定位

MTGNissa 当前采用以下运行模型：

- 单用户
- 单应用实例
- 单个人数据库
- 外部只读卡牌数据库
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
      │     └── 外置卡牌数据库，只读
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
| 连接池 | Go 标准库 `database/sql` |
| 基础 SQL 扩展 | `sqlx` |
| 数据库迁移 | Goose，仅用于个人数据库 |
| 日志 | 标准库 `slog` |
| 配置 | 环境变量 |
| 依赖管理 | Go Modules |
| 依赖注入 | 构造函数手工注入 |
| 部署 | 单 Docker 镜像 |
| 前端托管 | 前端确定后使用 `embed` 嵌入构建产物 |

以下选项等待数据模型和需求评估后确定：

- 是否使用 `sqlc`
- 是否使用 `goqu` 或其他动态 SQL Builder
- 前端框架
- OpenAPI 生成方案
- 业务模块划分
- API 设计

SQL 工具根据实际查询形态选择：

```text
固定 SQL 较多        → sqlc
动态筛选较多        → sqlx + SQL Builder
两种场景同时存在    → 按数据源或查询类型组合使用
```

## 数据库边界

### 卡牌数据库

- 外部部署
- 使用独立连接配置
- 使用独立只读账号
- 应用不执行迁移
- 应用不修改表结构
- 应用不写入数据

卡牌查询以 `scryfall_card` 为主体。具体印刷及语言版本使用 `scryfall_id`，逻辑卡牌使用 `oracle_id`；牌面级展示和翻译关联继续使用 `uuid`、`face_oracle_id` 与 `face_index`。

### 个人数据库

- 外部部署
- 一个实例对应一个个人数据库
- 应用拥有读写权限
- 由 Goose 管理结构迁移
- 保存当前实例所有者的数据

个人数据库的表结构和一致性规则见 [个人数据库设计](app-data-schema.md)。当前单实例单用户，不创建占位 `users` 表或固定用户记录。

## 连接池设计

应用启动时创建两个长期存在的数据库连接池句柄：

```go
type Databases struct {
	Card *sqlx.DB
	App  *sqlx.DB
}
```

`*sql.DB` 和 `*sqlx.DB` 表示连接池，不表示一条固定的 MySQL 连接。连接由连接池按需创建、复用和回收。

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
	Router *gin.Engine
	CardDB *sqlx.DB
	AppDB  *sqlx.DB
}
```

当前阶段不使用全局数据库变量、自动扫描、反射式 IoC 或依赖注入容器。

## 初始目录结构

```text
mtgnissa/
├── cmd/
│   └── server/
│       └── main.go
├── docs/
│   └── architecture.md
├── internal/
│   ├── bootstrap/
│   ├── config/
│   ├── database/
│   ├── health/
│   └── httpapi/
├── migrations/
├── web/
├── .gitignore
├── README.md
└── go.mod
```

当前目录仅表达基础设施职责。业务评估完成前，不创建收藏、卡组或其他领域包。

## HTTP 基础能力

基础 HTTP 层计划提供：

- Panic Recovery
- Request ID
- 访问日志
- 统一错误响应
- 请求超时
- 请求体大小限制
- 开发阶段需要的 CORS
- `/health/live`
- `/health/ready`

`/health/live` 只用于判断进程和 HTTP 服务是否正常。`/health/ready` 用于判断初始化是否完成；数据库检查策略在实现部署流程时确定。

## 生命周期

启动顺序：

```text
读取环境变量
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

基础环境变量计划包括：

```text
HTTP_ADDR
LOG_LEVEL

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

连接池参数分别使用 `CARD_DB_*` 和 `APP_DB_*` 前缀配置。

数据库密码不得写入 Git 仓库、Dockerfile、Docker 镜像、默认配置文件或日志。

## Docker 部署

最终镜像只包含 Go 可执行文件、前端构建产物以及运行所需的 CA 证书和时区数据。

容器要求：

- 使用非 root 用户运行
- 保持无状态
- 不绑定业务数据卷
- 日志输出到标准输出
- 通过环境变量读取配置
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
- 卡牌数据库只读
- 单实例对应单个人数据库
- 使用 `database/sql` 连接池
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
