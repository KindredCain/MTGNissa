# MTGNissa

## 项目介绍

MTGNissa 是一个面向个人部署的万智牌管理 Web 服务。

服务采用 Go HTTP 服务和双 MySQL 数据源，支持个人数据库自动迁移，以及从固定目录全量加载卡牌数据。Card DB 在日常业务中只读，显式启用的数据加载功能可以全量替换其固定白名单表；App DB 保存收藏、愿望单、卡组和操作日志。

## 部署

推荐复制 [`config.example.yaml`](config.example.yaml) 到部署主机，并以只读方式挂载：

```bash
docker run \
  -v /srv/mtgnissa/config.yaml:/config/mtgnissa.yaml:ro \
  -v /srv/mtgnissa/card-data:/data/card-data:ro \
  your-image -config /config/mtgnissa.yaml
```

也可以设置 `CONFIG_FILE=/config/mtgnissa.yaml`，此时不需要传 `-config`。命令行参数优先于 `CONFIG_FILE`。

加载顺序为：内置默认值 → YAML 文件 → 环境变量。可选的 `app.display_name` 用于页面标识当前是谁的卡牌库，默认是空字符串，也可由 `APP_DISPLAY_NAME` 覆盖。`app.timezone` 是页面时间展示、操作日志自然日查询及每日汇总的权威时区，使用 IANA 时区名称，默认是 `UTC`，也可由 `APP_TIMEZONE` 覆盖；它不修改数据库会话时区。数据库密码等敏感值可以不写入文件，而通过 `CARD_DB_PASSWORD`、`APP_DB_PASSWORD` 或 Docker Secret 注入。原有的 `HTTP_ADDR`、`CARD_DB_*`、`APP_DB_*`、`CARD_DATA_*` 环境变量仍然可用。

卡牌数据加载能力默认关闭。需要使用时，在 YAML 中配置只读数据目录并显式启用：

```yaml
card_data:
  dir: /data/card-data
  load_enabled: true
```

该目录必须包含以下九个文件，具体字段和处理规则见[新数据字典](docs/card-data-dictionary.md)：

- 标准 JSON：`AllPrintings.json`，提供普通卡、Token、系列、英文裁定和赛制等元数据。
- 旧 NDJSON：`scryfall_card.json`、`zhs_card.json`、`zhs_flavor.json`、`zhs_oracle.json`、`zhs_ruling.json`、`zhs_set.json`、`zhs_type.json`；其中 `scryfall_card.json` 只用于旧翻译 UUID 映射，不提供卡牌元数据。
- 标准 JSONL：`oracle-tags.jsonl`。

加载器按文件真实格式流式读取；旧 NDJSON 按字典移除一层重复转义，标准 JSON／JSONL 不应用该处理。`rulings.jsonl` 已废弃，不再需要。预览系列及其子系列整体排除，正式 Card DB 按新字典固定白名单原子替换。

## 接口说明

本节说明健康检查与卡牌数据加载入口。具体业务接口统一见 [API 契约](docs/api-contract.md)。

### 健康检查

```text
GET /health/live
GET /health/ready
```

### 卡牌数据加载

启动全量加载任务：

```http
POST /api/v1/card_data/load
Content-Type: application/json

{"confirmation":"LOAD_CARD_DATA"}
```

成功返回 `202` 和任务 ID；同一时间已有任务时返回 `409`。任务状态可通过以下接口查询：

```http
GET /api/v1/card_data/load/{task_id}
```

卡牌加载是维护接口，只操作 Card DB，不写入个人数据库，也不计入用户操作日志。完整校验、原子换表和安全限制见功能需求文档。

## 文档索引

- [功能需求](docs/requirements.md)：页面行为、查询规则、卡组规则、日志规则和卡牌数据加载要求。
- [API 契约](docs/api-contract.md)：接口路径、请求参数、响应 DTO、错误码、分页与服务端排序约定。
- [搜索语法支持矩阵](docs/search-syntax-support.md)：首版必须实现的查询关键字、别名、比较符和值域。
- [技术架构设计](docs/architecture.md)：系统边界、技术选型、生命周期、配置与部署约束。
- [卡牌数据字典](docs/card-data-dictionary.md)：原始卡牌文件、字段含义、关联关系和 Card DB 物理模型。
- [个人数据库设计](docs/app-data-schema.md)：App DB 表结构、索引、一致性规则和迁移落地方式。
- [原型与页面说明](design/README.md)：原型文件入口、页面说明索引及统一交互约定。

## 数据来源

- `AllPrintings.json`：[MTGJSON](https://mtgjson.com/)
- `oracle-tags.jsonl`：[Scryfall Bulk Data API](https://scryfall.com/docs/api/bulk-data)
- `scryfall_card.json` 及其他 `zhs_*` 数据：[HeliumOctahelide/magic-cards-zhs](https://github.com/HeliumOctahelide/magic-cards-zhs)
