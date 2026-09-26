# MTGNissa

## 项目介绍

MTGNissa 是一个面向个人部署的万智牌管理 Web 服务。

当前后端提供 Go HTTP 服务、双 MySQL 数据源连接、个人数据库自动迁移，以及从固定目录全量加载卡牌数据的维护接口。Card DB 在日常业务中只读，显式启用的维护接口可以全量替换其固定白名单表；App DB 保存收藏、愿望单、卡组和操作日志。

## 部署

推荐复制 [`config.example.yaml`](config.example.yaml) 到部署主机，并以只读方式挂载：

```bash
docker run \
  -v /srv/mtgnissa/config.yaml:/config/mtgnissa.yaml:ro \
  -v /srv/mtgnissa/card-data:/data/card-data:ro \
  your-image -config /config/mtgnissa.yaml
```

也可以设置 `CONFIG_FILE=/config/mtgnissa.yaml`，此时不需要传 `-config`。命令行参数优先于 `CONFIG_FILE`。

加载顺序为：内置默认值 → YAML 文件 → 环境变量。可选的 `app.display_name` 用于页面标识当前是谁的卡牌库，默认是空字符串，也可由 `APP_DISPLAY_NAME` 覆盖。数据库密码等敏感值可以不写入文件，而通过 `CARD_DB_PASSWORD`、`APP_DB_PASSWORD` 或 Docker Secret 注入。原有的 `HTTP_ADDR`、`CARD_DB_*`、`APP_DB_*`、`CARD_DATA_*` 环境变量仍然可用。

卡牌数据加载能力默认关闭。需要使用时，在 YAML 中配置只读数据目录并显式启用：

```yaml
card_data:
  dir: /data/card-data
  load_enabled: true
```

该目录必须包含功能需求文档约定的九个 NDJSON/JSONL 文件，包括 `rulings.jsonl` 和 `oracle-tags.jsonl`。

## 接口说明

以下为当前已经实现的 HTTP 接口；完整业务接口会随功能实现继续补充。

### 健康检查

```text
GET /health/live
GET /health/ready
```

### 卡牌数据加载

启动全量加载任务：

```http
POST /api/v1/card-data/load
Content-Type: application/json

{"confirmation":"LOAD_CARD_DATA"}
```

成功返回 `202` 和任务 ID；同一时间已有任务时返回 `409`。任务状态可通过以下接口查询：

```http
GET /api/v1/card-data/load/{task-id}
```

卡牌加载是维护接口，只操作 Card DB，不写入个人数据库，也不计入用户操作日志。完整校验、原子换表和安全限制见功能需求文档。

## 文档索引

- [功能需求](docs/requirements.md)：页面行为、查询规则、卡组规则、日志规则和卡牌数据加载要求。
- [技术架构设计](docs/architecture.md)：系统边界、技术选型、生命周期、配置与部署约束。
- [卡牌数据字典](docs/card-data-dictionary.md)：原始卡牌文件、字段含义、关联关系和 Card DB 物理模型。
- [个人数据库设计](docs/app-data-schema.md)：App DB 表结构、索引、一致性规则和迁移落地方式。
