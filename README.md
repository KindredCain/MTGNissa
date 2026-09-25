# MTGNissa

MTGNissa 是一个面向个人部署的万智牌管理 Web 服务。

当前后端已提供 Go HTTP 服务骨架，以及从固定目录全量重建卡牌数据库的维护接口。

## 运行

推荐复制 [`config.example.yaml`](config.example.yaml) 到部署主机，并以只读方式挂载：

```bash
docker run \
  -v /srv/mtgnissa/config.yaml:/config/mtgnissa.yaml:ro \
  -v /srv/mtgnissa/card-data:/data/card-data:ro \
  your-image -config /config/mtgnissa.yaml
```

也可以设置 `CONFIG_FILE=/config/mtgnissa.yaml`，此时不需要传 `-config`。命令行参数优先于 `CONFIG_FILE`。

加载顺序为：内置默认值 → YAML 文件 → 环境变量。数据库密码等敏感值可以不写入文件，而通过 `CARD_DB_PASSWORD`、`APP_DB_PASSWORD` 或 Docker Secret 注入。原有的 `HTTP_ADDR`、`CARD_DB_*`、`APP_DB_*`、`CARD_DATA_*` 环境变量仍然可用。

健康检查：

```text
GET /health/live
GET /health/ready
```

## 卡牌数据重建

该能力默认关闭，可在 YAML 中启用：

```yaml
card_data:
  dir: /data/card-data
  rebuild_enabled: true
```

`CARD_DATA_DIR` 必须包含需求文档约定的七个 NDJSON 文件。启动任务：

```http
POST /api/v1/card-data/rebuild
Content-Type: application/json

{"confirmation":"REBUILD_CARD_DATABASE"}
```

成功返回 `202` 和任务 ID；同一时间已有任务时返回 `409`。任务状态可通过以下接口查询：

```http
GET /api/v1/card-data/rebuild/{task-id}
```

项目文档：

- [技术架构设计](docs/architecture.md)
- [功能需求](docs/requirements.md)
- [卡牌数据字典](docs/card-data-dictionary.md)
