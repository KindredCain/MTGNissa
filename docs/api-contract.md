# MTGNissa API 契约

## 1. 文档状态

- 状态：首版业务 API 契约评审稿
- 契约版本：`v1`
- 基础路径：`/api/v1`
- 数据格式：`application/json; charset=utf-8`
- 需求基线：[功能需求](requirements.md)与[搜索语法支持矩阵](search-syntax-support.md)
- 边界：本文定义 HTTP 路径、请求响应 DTO、分页、并发和错误代码；业务计算及页面交互仍以功能需求和 `design/` 为准。

本期不实现认证。业务 API 与维护接口均应只部署在可信网络或受访问控制的反向代理之后。

## 2. 通用约定

### 2.1 响应信封

所有响应，包括成功、校验失败、未找到路由和服务端异常，都使用统一信封。HTTP 状态码必须与 `status` 相同。

成功：

```json
{
  "status": 200,
  "data": {"deck_id": "12", "name": "Superfriends", "version": "3"},
  "error": null,
  "request_id": "20261009T120000.000000000"
}
```

失败：

```json
{
  "status": 422,
  "data": null,
  "error": {
    "code": "validation_failed",
    "message": "deck name must not be empty",
    "details": [
      {"field": "name", "reason": "required"}
    ]
  },
  "request_id": "20261009T120000.000000000"
}
```

`error.details` 是可选字段，仅用于客户端需要定位具体字段或卡组代码行的错误；不得包含 SQL、堆栈、文件系统路径或内部异常对象。

信封字段约束：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| `status` | integer | 否 | 与实际 HTTP 状态码完全相同 |
| `data` | endpoint DTO | 是 | 成功时为对应 DTO，失败时固定为 `null` |
| `error` | `ErrorDTO` | 是 | 成功时固定为 `null`，失败时必填 |
| `request_id` | string | 否 | 本次请求最终 ID，最长 128 字符 |

`ErrorDTO` 必须包含非空 `code` 和安全的 `message`。可选 `details` 是对象数组；字段校验项使用 `field` 与 `reason`，代码解析项使用从 1 开始的 `line`、`reason` 和可选的安全输入摘要 `input`。同一项不得同时返回 `field` 和 `line`。`field` 使用从请求 DTO 根开始的点号与零基数组下标格式，例如 `sections[1].cards[3].scryfall_id`、`scryfall_ids[2]`；不得混用 JSON Pointer、美元符号路径或一基数组下标。

下文各接口的响应示例为统一信封中的 `data` 内容，实际 HTTP 响应仍必须包裹完整信封。

### 2.2 请求约定

- JSON 字段、路径参数、查询参数以及前端 API 成员名统一使用 `snake_case`；例如路由动作 `parse_code` 在前端同名表示为 `parse_code`。
- Go 导出类型、字段和处理函数统一使用 `UpperCamelCase`，按单词边界映射上述名称，例如 `parse_code` 对应 `ParseCode`，`scryfall_id` 对应 `ScryfallID`。
- JSON 请求对象拒绝未知字段、尾随的第二个 JSON 值和错误类型。
- 除非端点正文明确标记为可选，请求示例和请求字段表中的字段均为必填。缺少必填字段返回 `422 validation_failed` 并在 `error.details[].field` 中定位；未知字段、错误 JSON 类型或多余 JSON 值返回 `400 invalid_request`。
- 查询参数同样拒绝未在当前端点声明的名称；例如擅自传入 `limit` 返回 `400 invalid_request`，不得静默忽略。可选查询参数会在各端点明确标记，重复参数只允许用于声明为数组的字段。
- 客户端可发送不超过 128 字符的 `X-Request-ID`；服务端始终在响应头和信封中返回最终请求 ID。
- 时间戳在数据库中统一以 UTC 保存，并在接口中使用 RFC 3339 UTC 表示，例如 `2026-10-09T04:00:00Z`；自然日使用 `YYYY-MM-DD`，其时区语义由具体端点声明。
- 业务资源标识作为不透明字符串返回；前端只用于回传、路由和缓存键，不解析其来源或结构。
- MySQL `BIGINT UNSIGNED` 标识和版本统一按十进制字符串返回，避免 JavaScript 精度丢失。
- 多选查询参数使用重复参数，例如 `language=en&language=ja`，不使用逗号拼接。
- 空白查询关键字按未提供处理。除 Set 名称和卡组名称查询外，所有卡名模糊查询都直接对 `scryfall_card.name` 执行去除首尾空白后、不区分大小写的文字包含匹配；不查询翻译名、`printed_name` 或 `flavor_name`，用户输入中的数据库通配符按普通文字转义。命中具体记录后再应用各接口自己的范围和去重规则。
- 除卡牌数据维护任务外，会改变业务状态的 `POST` 必须携带 `Idempotency-Key` 请求头。客户端为一次用户意图生成一个 RFC 4122 UUID，并使用 36 字符规范文本形式；同一次请求发生超时或连接中断时复用原值，新的用户操作必须生成新值。
- 服务端必须持久化幂等键。请求指纹由方法、含实际资源标识的规范化路径和解码后 DTO 的规范 JSON 计算，JSON 空白和对象字段顺序不影响指纹。相同请求使用同一键重放时不得再次改变状态或重复写日志，返回首次请求已经提交的相同 HTTP 状态、`data` 和 `Location`，并增加响应头 `Idempotency-Replayed: true`；响应信封使用本次请求的新 `request_id`。同一键对应不同请求时返回 `409 idempotency_conflict`。
- 两个相同幂等请求并发到达时只能有一个执行 mutation；另一个等待首个事务提交后按重放处理。首个事务回滚时不保留已完成幂等结果，后续重试可以重新执行。
- 只持久化已经成功提交的业务结果。请求解析、字段校验、版本冲突、依赖故障和内部错误不占用幂等键；客户端修正请求后应使用新的幂等键提交。
- 需要幂等键的接口包括收藏调整、愿望单状态、创建／保存／复制／删除卡组。只读 `GET`、只读的卡组编辑上下文批量查询、卡组代码解析和卡牌数据维护任务不使用该键。

### 2.3 游标分页

Set、全局卡牌搜索、我的卡牌、愿望单、卡组列表和增加卡牌搜索使用服务端游标：

```json
{
  "items": [],
  "total": 0,
  "next_cursor": null
}
```

- `cursor` 是不透明字符串，客户端不得解析、拼接或持久修改。
- 游标绑定生成它的搜索、筛选、合并和排序条件；条件改变后必须丢弃旧游标。
- `next_cursor = null` 表示已经加载完毕。
- 单批数量由服务端配置和联调结果确定，本契约不允许客户端指定 `limit`。
- 无结果返回 `200`、空 `items`、`total = 0` 和 `next_cursor = null`。

具体 Set 卡牌列表一次返回当前 Set、语言和名称关键字下的完整轻量结果，不使用服务端游标。操作日志使用固定每页 10 条的页码分页，不使用游标。

### 2.4 公共枚举

```text
finish_kind: normal | foil
deck_section: commander | mainboard | sideboard | maybeboard
operation_type:
  add_card | remove_card | add_wishlist | remove_wishlist |
  create_deck | delete_deck | copy_deck | modify_deck
color_group: W | U | B | R | G | multicolor | colorless
mana_value_group: 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7_plus
rarity: common | uncommon | rare | mythic | special | bonus
```

公开工艺枚举有意只保留 `normal` 与 `foil`。Card DB 中的普通工艺映射为 `normal`；传统闪和蚀刻闪都映射为 `foil`，收藏数量、日志和 `available_finishes` 不再区分传统闪与蚀刻闪。只有蚀刻闪而没有传统闪的印刷仍必须在 `available_finishes` 中返回 `foil`。

### 2.5 公共 DTO

`CardImageDTO`：

```json
{
  "front_url": "https://cards.scryfall.io/normal/front/...jpg",
  "back_url": null,
  "has_distinct_back": false
}
```

`SetRefDTO`：

```json
{
  "set_id": "00000000-0000-0000-0000-000000000000",
  "code": "khm",
  "name": "Kaldheim",
  "display_name": "凯德海姆",
  "released_at": "2021-02-05"
}
```

`CardSummaryDTO` 是卡牌列表、搜索结果和卡组卡图共同使用的轻量展示对象：

```json
{
  "scryfall_id": "00000000-0000-0000-0000-000000000000",
  "oracle_id": "00000000-0000-0000-0000-000000000010",
  "name": "Nissa, Vital Force",
  "display_name": "生机妮莎",
  "set": {
    "set_id": "00000000-0000-0000-0000-000000000001",
    "code": "kld",
    "name": "Kaladesh",
    "display_name": "卡拉德许",
    "released_at": "2016-09-30"
  },
  "collector_number": "163",
  "language": {"code": "ja", "name": "日文", "sort_order": 3},
  "released_at": "2016-09-30",
  "cmc": 5,
  "mana_cost": "{3}{G}{G}",
  "colors": ["G"],
  "rarity": "mythic",
  "layout": "normal",
  "images": {
    "front_url": "https://cards.scryfall.io/normal/front/...jpg",
    "back_url": null,
    "has_distinct_back": false
  }
}
```

`scryfall_id` 是具体印刷及语言版本的公开标识，也是所有卡牌详情、收藏、愿望单和卡组请求使用的卡牌标识。多牌面共享同一个 `scryfall_id`，由 `face_index` 区分牌面。客户端必须使用服务端返回的规范 UUID 原值，不得自行拼接或替换版本。

`oracle_id` 是逻辑卡牌标识，作为只读属性随卡牌对象返回；数据源没有逻辑身份时为 `null`。前端可用它进行本地去重、合并和同卡判断，但不得把它放入路径、查询参数或 mutation 请求，所有后端请求仍提交对应的 `scryfall_id`。后端负责将其解析为 `oracle_id` 后执行逻辑卡查询、统计和卡组持久化。`deck_card` 没有独立 ID，其数据库身份固定为 `(deck_id, section_type, oracle_id)`；API 不定义 `deck_card_id`。`set_id` 是独立的 Set 标识，不能与上述两个卡牌标识互换。

公共 DTO 的完整字段约束：

| DTO | 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- | --- |
| `LanguageDTO` | `code` | string | 否 | 语言代码 |
|  | `name` | string | 否 | 与前端全站固定语言代码映射一致的 UI 展示名称；日志响应只返回代码 |
|  | `sort_order` | integer | 否 | 字典顺序 |
| `CardImageDTO` | `front_url` | string | 是 | 数据源没有卡图时为 `null`，客户端使用统一占位图 |
|  | `back_url` | string | 是 | 没有独立背面时为 `null` |
|  | `has_distinct_back` | boolean | 否 | 是否显示翻面控件 |
| `SetRefDTO` | `set_id` | string | 否 | 不透明 Set 标识 |
|  | `code` | string | 否 | 系列代码 |
|  | `name` | string | 否 | 英文名称 |
|  | `display_name` | string | 否 | 翻译后名称，缺失时为英文名称 |
|  | `released_at` | date | 是 | 数据源缺失时为 `null` |
| `CardSummaryDTO` | `scryfall_id` | string | 否 | 具体印刷及语言版本标识，也是详情入口 |
|  | `oracle_id` | string | 是 | 只读逻辑卡牌标识；数据源缺失时为 `null` |
|  | `name` | string | 否 | 英文组合卡名 |
|  | `display_name` | string | 否 | 翻译后组合卡名，缺失时回退英文 |
|  | `set` | `SetRefDTO` | 否 | 当前展示系列 |
|  | `collector_number` | string | 否 | 收藏编号原文 |
|  | `language` | `LanguageDTO` | 否 | 当前展示语言 |
|  | `released_at` | date | 是 | 发行日期 |
|  | `cmc` | number | 否 | 法术力值 |
|  | `mana_cost` | string | 是 | 无费用时为 `null` |
|  | `colors` | string[] | 否 | `W/U/B/R/G`，无色为空数组 |
|  | `rarity` | string | 否 | 卡牌稀有度原值 |
|  | `layout` | string | 否 | 数据源布局代码 |
|  | `images` | `CardImageDTO` | 否 | 卡图地址及翻面能力 |

所有 DTO 属性本身都必须出现；“可空”只表示属性值可以为 `null`，不表示可以省略属性。空集合使用 `[]`，不得以 `null` 代替。具名字段表优先于示例；没有单独字段表的端点以成功示例中的全部键作为完整属性集，JSON 值类型即字段类型，只有示例明确使用 `null` 或正文明确声明可空的字段才允许 `null`。示例中的 `...` 只是字符串内容缩写，不表示省略属性；空 `items` 只表示合法零结果，其元素类型由相邻正文指定。

### 2.6 输入边界

| 输入 | 约束 |
| --- | --- |
| `scryfall_id`、`set_id` | 规范 UUID 字符串，只能使用服务端此前返回的原值 |
| 响应中的 `oracle_id` | 规范 UUID 字符串或 `null`；只读，不能作为请求参数 |
| `deck_id`、`version` | 十进制正整数字符串，最长 20 位 |
| 列表和搜索 `q` | 去除首尾空白后最长 512 个 Unicode 字符；全空白按未提供处理，全局搜索的非空 `q` 必填 |
| 卡组名称 | 去除首尾空白后 1–255 个 Unicode 字符；保存规范化后的值 |
| 卡组代码 | UTF-8 文本，最长 768 KiB、最多 10,000 行 |
| JSON 请求体 | 全局最大 1 MiB；超过限制返回 `413 request_too_large` |
| 单条卡组数量 | 1–4,294,967,295 的整数；同分区合并后仍不得超过上限 |
| 收藏 `delta` | 只能为 `1` 或 `-1` |
| 多选参数 | 单个字段去重后最多 128 项；空数组等同未提供 |
| 页码 | 十进制整数，最小为 1；超过末页返回空 `items` 而不是错误 |

字符串不得包含 NUL。违反边界返回 `422 validation_failed`；JSON 结构、参数类型或枚举错误返回 `400 invalid_request`。

## 3. 接口总览

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/v1/app` | 应用展示配置 |
| `GET` | `/api/v1/search` | 全局名称／语法搜索 |
| `GET` | `/api/v1/sets/options` | Set 列表筛选项 |
| `GET` | `/api/v1/sets` | Set 无限列表 |
| `GET` | `/api/v1/cards` | 当前 Set、语言的完整轻量卡牌列表 |
| `GET` | `/api/v1/cards/{scryfall_id}` | 卡牌详情 |
| `POST` | `/api/v1/cards/{scryfall_id}/collection_adjustments` | 调整普通或闪卡数量 |
| `POST` | `/api/v1/cards/{scryfall_id}/wishlist_state` | 设置愿望单状态 |
| `GET` | `/api/v1/collection/options` | 我的卡牌筛选项 |
| `GET` | `/api/v1/collection` | 我的卡牌无限列表 |
| `GET` | `/api/v1/wishlist/options` | 愿望单筛选项 |
| `GET` | `/api/v1/wishlist` | 愿望单无限列表 |
| `GET` | `/api/v1/decks` | 卡组无限列表 |
| `POST` | `/api/v1/decks` | 创建空白卡组 |
| `GET` | `/api/v1/decks/{deck_id}` | 卡组详情／编辑初始数据 |
| `POST` | `/api/v1/decks/{deck_id}/save` | 保存完整卡组编辑态 |
| `POST` | `/api/v1/decks/{deck_id}/delete` | 删除卡组 |
| `POST` | `/api/v1/decks/{deck_id}/copies` | 复制卡组 |
| `POST` | `/api/v1/decks/{deck_id}/parse_code` | 在卡组编辑页解析代码但不入库 |
| `GET` | `/api/v1/decks/{deck_id}/cards/search` | 搜索可加入当前卡组的卡牌 |
| `POST` | `/api/v1/decks/{deck_id}/card_contexts` | 按需批量取得未保存编辑态所需的卡牌上下文 |
| `GET` | `/api/v1/operation_logs/summary` | 操作日志每日类型汇总 |
| `GET` | `/api/v1/operation_logs` | 指定日期和类型的操作明细 |
| `POST` | `/api/v1/card_data/load` | 启动卡牌数据加载 |
| `GET` | `/api/v1/card_data/load/{task_id}` | 查询加载任务 |

健康检查继续使用 `/health/live` 和 `/health/ready`，不放在 `/api/v1` 下。

## 4. 应用元信息

### 4.1 `GET /api/v1/app`

响应 `200`：

```json
{
  "display_name": "Nissa",
  "timezone": "Asia/Shanghai"
}
```

`display_name` 按配置原值返回，允许空字符串。`timezone` 必须是后端可加载的 IANA 时区名称，是全站解释业务自然日的权威时区；配置无效时服务不得启动，不得静默回退到服务器本地时区或 UTC。

## 5. Set 与全部卡牌

### 5.1 `GET /api/v1/sets/options`

返回 Set 列表使用的动态筛选项：

```json
{
  "languages": [
    {"code": "en", "name": "英文", "sort_order": 1}
  ],
  "set_types": ["commander", "core", "expansion"]
}
```

`languages` 固定按 `sort_order` 升序只返回前 11 项；`set_types` 返回当前 Card DB 中全部非空去重值并稳定排序。

### 5.2 `GET /api/v1/sets`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 系列名称或代码，不区分大小写的包含匹配 |
| `language` | string[] | 同一字段 OR；只保留至少存在一种所选语言的 Set |
| `set_type` | string[] | 同一字段 OR |
| `sort` | enum | `released_desc`（默认）、`released_asc`、`name_asc`、`name_desc`、`code_asc`、`code_desc`、`card_count_desc`、`card_count_asc` |
| `cursor` | string? | 下一批游标 |

`language` 与 `set_type` 之间使用 AND。

列表项：

```json
{
  "set_id": "...",
  "code": "khm",
  "name": "Kaldheim",
  "display_name": "凯德海姆",
  "set_type": "expansion",
  "card_count": 405,
  "released_at": "2021-02-05",
  "languages": [
    {"code": "en", "name": "英文", "sort_order": 1, "available": true}
  ],
  "default_language": "en",
  "parent_set_id": null
}
```

`default_language` 必须是返回语言数组中从左到右第一项 `available = true` 的代码；不得使用隐藏的语言优先级。不能可靠确定父子关系时 `parent_set_id` 返回 `null`。

`SetListItemDTO` 的 `set_id`、`code`、`name`、`display_name`、`set_type`、`card_count` 和 `languages` 不可为 `null`；`released_at`、`default_language`、`parent_set_id` 可为 `null`。`card_count` 为非负整数。`languages` 元素在 `LanguageDTO` 基础上增加不可空的 `available: boolean`；没有任何可用的前 11 种语言时 `default_language = null`，整行不提供跳转目标。

### 5.3 `GET /api/v1/cards`

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `set_id` | 是 | 当前 Set 标识 |
| `language` | 是 | 当前卡牌原语言代码 |
| `q` | 否 | 当前 Set 内按 `scryfall_card.name` 模糊查询，不解析搜索语法 |

响应一次返回完整轻量结果：

```json
{
  "set": {
    "set_id": "00000000-0000-0000-0000-000000000001",
    "code": "kld",
    "name": "Kaladesh",
    "display_name": "卡拉德许",
    "released_at": "2016-09-30"
  },
  "language": {"code": "en", "name": "英文", "sort_order": 1},
  "items": [],
  "total": 0
}
```

每个 `items` 元素为 `CardSummaryDTO`。前端在完整结果上执行颜色、法术力、稀有度筛选和收藏编号／法术力／名称排序，并自行分批创建卡图节点。

### 5.4 `GET /api/v1/cards/{scryfall_id}`

返回当前 `scryfall_id` 指向的具体印刷及语言版本详情。`scryfall_id` 来自任一列表、搜索、卡组或版本选项响应：

```json
{
  "card": {
    "scryfall_id": "00000000-0000-0000-0000-000000000000",
    "oracle_id": "00000000-0000-0000-0000-000000000010",
    "name": "Nissa, Vital Force",
    "display_name": "生机妮莎",
    "set": {
      "set_id": "00000000-0000-0000-0000-000000000001",
      "code": "kld",
      "name": "Kaladesh",
      "display_name": "卡拉德许",
      "released_at": "2016-09-30"
    },
    "collector_number": "163",
    "language": {"code": "ja", "name": "日文", "sort_order": 3},
    "released_at": "2016-09-30",
    "cmc": 5,
    "mana_cost": "{3}{G}{G}",
    "colors": ["G"],
    "rarity": "mythic",
    "layout": "normal",
    "images": {"front_url": "https://...", "back_url": null, "has_distinct_back": false}
  },
  "faces": [
    {
      "face_index": 0,
      "name": "Nissa, Vital Force",
      "display_name": "生机妮莎",
      "mana_cost": "{3}{G}{G}",
      "type_line": "Legendary Planeswalker — Nissa",
      "display_type_line": "传奇鹏洛客～妮莎",
      "oracle_text": "...",
      "display_oracle_text": "...",
      "flavor_text": null,
      "display_flavor_text": null,
      "power": null,
      "toughness": null,
      "loyalty": "5",
      "image_url": "https://..."
    }
  ],
  "collection": {"normal_quantity": 0, "foil_quantity": 1},
  "logical_collection": {"normal_quantity": 2, "foil_quantity": 1, "total_quantity": 3},
  "wishlist": {"included": true},
  "available_finishes": ["normal", "foil"],
  "rulings": [
    {"published_at": "2016-09-20", "source": "WotC", "text": "..."}
  ],
  "oracle_tags": [
    {"tag_id": "...", "label": "Ramp", "query": "otag:\"Ramp\""}
  ],
  "languages": [
    {
      "scryfall_id": "00000000-0000-0000-0000-000000000002",
      "oracle_id": "00000000-0000-0000-0000-000000000010",
      "language": {"code": "en", "name": "英文", "sort_order": 1},
      "normal_quantity": 0,
      "foil_quantity": 0
    }
  ],
  "other_versions": [
    {
      "set": {"set_id": "00000000-0000-0000-0000-000000000004", "code": "cmm", "name": "Commander Masters", "display_name": "指挥官大师", "released_at": "2023-08-04"},
      "collector_number": "307",
      "released_at": "2023-08-04",
      "normal_quantity": 0,
      "foil_quantity": 0,
      "languages": [
        {
          "scryfall_id": "00000000-0000-0000-0000-000000000003",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "language": {"code": "en", "name": "英文", "sort_order": 1},
          "normal_quantity": 0,
          "foil_quantity": 0
        }
      ]
    }
  ],
  "deck_usage": {
    "total_decks": 2,
    "decks": [
      {"deck_id": "12", "name": "Superfriends", "quantity": 1, "sections": ["mainboard"]}
    ]
  }
}
```

`CardFaceDTO` 字段约束：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| `face_index` | integer | 否 | 从 0 开始且在当前详情内连续 |
| `name`、`display_name` | string | 否 | 原名及按语言回退后的展示名 |
| `mana_cost` | string | 是 | 该牌面没有费用时为 `null` |
| `type_line`、`display_type_line` | string | 是 | 数据源缺失时为 `null` |
| `oracle_text`、`display_oracle_text` | string | 是 | 没有规则叙述时为 `null` |
| `flavor_text`、`display_flavor_text` | string | 是 | 没有背景叙述时为 `null` |
| `power`、`toughness`、`loyalty` | string | 是 | 不适用或缺失时为 `null`，保留原始符号文本 |
| `image_url` | string | 是 | 当前牌面没有独立图片时为 `null` |

详情顶层的 `card` 为 `CardSummaryDTO`，`faces` 至少一项且按 `face_index` 排序。`collection` 表示当前 `scryfall_id` 的数量；`logical_collection` 表示同一 `oracle_id` 下全部系列、收藏编号和语言的权威合计，没有 `oracle_id` 时只汇总当前 `scryfall_id`。上述数量均为非负整数。`wishlist.included` 为 boolean；`available_finishes` 只含 `finish_kind` 且至少一项，数据源的蚀刻闪能力按公共映射返回 `foil`。`rulings`、`oracle_tags`、`languages`、`other_versions` 和 `deck_usage.decks` 无结果时返回 `[]`。

- `languages` 返回当前系列和收藏编号下的实际语言选项；每项包含可直接进入详情的 `scryfall_id`、语言和当前拥有数量。
- `other_versions` 以 `set_id + collector_number` 为分组键，排除当前正在查看的版本分组，不按语言重复版本行。每项返回系列、收藏编号、该版本全部语言的拥有数量合计，以及实际存在的完整 `languages` 数组；语言项携带可直接进入详情的 `scryfall_id`。版本按需求中的发行日期规则排序，发布日期相同时按 Set 缩写、收藏编号和内部稳定卡牌标识兜底。
- `rulings`、`oracle_tags` 和 `deck_usage` 随详情一次返回，不另设分页接口。
- `oracle_tags[].query` 由后端生成为可直接提交给全局搜索的规范查询，固定格式为 `otag:"<escaped label>"`。标签中的反斜杠先转义为 `\\`，双引号转义为 `\"`；即使标签不含空格也保留双引号。前端不得自行拼接、去引号或再次转义。
- 翻译字段已经由后端按“简体中文优先，缺失回退英文”生成；前端不再实现语言选择逻辑。

## 6. 全局搜索

### 6.1 `GET /api/v1/search`

该接口独立承载全局名称查询、完整语法解析、条件组合、去重、稳定排序和游标分页，不属于普通卡牌列表接口。

查询参数：`q` 必填，`cursor` 可选。

响应：

```json
{
  "input": "function:tutor",
  "mode": "syntax",
  "executed_query": "otag:tutor",
  "items": [],
  "total": 0,
  "next_cursor": null
}
```

- `mode` 为 `syntax` 或 `name`。
- 只有完整识别首版语法时才使用 `syntax`；否则将完整输入按 `scryfall_card.name` 模糊查询并返回 `name`，不得返回语法错误。
- 后端先按 `oracle_id` 去重，再按 `oracle_id` 稳定排序和生成游标；`oracle_id` 作为 `CardSummaryDTO` 的只读属性正常返回，游标本身仍是不透明字符串。
- 每个命中的 `oracle_id` 由后端选择任意一条具体记录作为 `CardSummaryDTO`、卡图和详情目标；不要求选择最新发行版本，也不保证重复查询或重新导入后仍选择相同版本。代表记录的版本、系列、语言和发行日期不得参与结果顺序计算，客户端不得再次选择展示记录。
- 全局搜索只以具有非空 `oracle_id` 的卡牌为逻辑结果；`oracle_id = null` 的记录不进入全局名称或语法搜索结果。具体 Set 卡牌列表仍可按 `scryfall_id` 展示这类记录。

## 7. 收藏与愿望单

### 7.1 `POST /api/v1/cards/{scryfall_id}/collection_adjustments`

请求：

```json
{
  "finish": "normal",
  "delta": 1
}
```

- `finish`、`delta` 必填；请求不接受合并方式或收藏列表筛选条件。
- `delta` 只能是 `1` 或 `-1`。
- 本接口始终只调整路径 `scryfall_id` 对应的具体印刷及语言记录，不得按 `oracle_id`、Set、收藏编号、语言筛选或收藏列表合并键扩大操作范围。增加和减少都作用于该记录的指定 `finish`；数量不足时返回 `409 collection_underflow`，不得改扣其他印刷或语言记录，也不得只应用部分增量。
- 卡牌详情页始终可以调用本接口。“我的卡牌”列表仅在 `merge_sets = false` 且 `merge_languages = false` 时显示加减控件；任一合并开关开启时卡位为只读合计，不得调用本接口。
- 后端必须锁定当前收藏记录，校验工艺可用性和新数量不小于 0，在同一 App DB 事务中更新收藏并写一条操作日志。
- 两种数量都归零时删除 `collection_item`；响应仍返回两个零值。

响应 `200`：

```json
{
  "scryfall_id": "00000000-0000-0000-0000-000000000000",
  "oracle_id": "00000000-0000-0000-0000-000000000010",
  "applied_delta": 1,
  "group": {
    "normal_quantity": 3,
    "foil_quantity": 1,
    "total_quantity": 4
  },
  "collection": {
    "total_quantity": 42,
    "distinct_cards": 30
  },
  "set_summary": {
    "set_id": "00000000-0000-0000-0000-000000000001",
    "total_quantity": 8,
    "distinct_cards": 6
  }
}
```

`group` 是路径 `scryfall_id` 这一具体印刷及语言记录的权威普通／闪卡数量，名称为兼容当前响应结构而保留，不表示合并列表分组；`collection` 是整个收藏的权威页头合计；`set_summary` 是该具体记录所属系列的权威汇总，卡牌缺少系列时为 `null`。两个 `distinct_cards` 都按 `oracle_id` 去重，缺少 `oracle_id` 时按各自内部卡牌标识独立计种，不受列表合并开关影响。成功响应不要求列表重查；调用方以这些对象更新当前未合并卡位或详情收藏面板、页头和当前可见的系列标头，并在本次页面会话中保持原卡图、位置和排序不变。具体记录的两种数量归零时也保留至重新进入页面；重新进入后再按权威列表重新选择展示卡和排序。

### 7.2 `POST /api/v1/cards/{scryfall_id}/wishlist_state`

请求以目标状态表达加入或移出愿望单：

```json
{"included": true}
```

- `included` 必填。
- `included = true` 表示加入，`false` 表示移出。
- 状态实际改变时，在同一事务中更新愿望单并写对应操作日志。
- 当前状态已经等于目标状态时仍返回成功，但不重复写日志。

响应 `200`：`{"scryfall_id":"00000000-0000-0000-0000-000000000000","oracle_id":"00000000-0000-0000-0000-000000000010","included":true}`。没有逻辑身份时 `oracle_id` 为 `null`。

### 7.3 `GET /api/v1/collection/options`

返回当前收藏数量大于零的动态筛选选项：

```json
{
  "sets": [{"set_id": "...", "code": "khm", "display_name": "凯德海姆"}],
  "languages": [{"code": "en", "name": "英文", "sort_order": 1}],
  "colors": ["W", "U", "B", "R", "G", "multicolor", "colorless"],
  "mana_values": ["0", "1", "2", "3", "4", "5", "6", "7_plus"],
  "rarities": ["common", "uncommon", "rare", "mythic", "special", "bonus"]
}
```

### 7.4 `GET /api/v1/collection`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 按关联的 `scryfall_card.name` 模糊查询 |
| `set_id`、`language`、`color`、`mana_value`、`rarity` | string[] | 同字段 OR，不同字段 AND |
| `sort` | enum | `set`（默认）、`color`、`mana_value`、`name` |
| `merge_sets` | boolean | 默认 `false`；`sort=set` 时必须为 `false` |
| `merge_languages` | boolean | 默认 `true` |
| `cursor` | string? | 下一批游标 |

收藏分组键固定如下。表中的括号表示带首项类型标记的概念元组，不是把字段直接拼接成一个字符串：

| `merge_sets` | `merge_languages` | `oracle_id` 非空时的分组键 | 分组语义 |
| --- | --- | --- | --- |
| `false` | `false` | `("print", scryfall_id)` | 每个具体 Set、收藏编号和语言记录独立展示 |
| `true` | `false` | `("oracle_language", oracle_id, language.code)` | 同一逻辑卡、同一语言跨 Set 和收藏编号合并 |
| `false` | `true` | `("oracle_printing", oracle_id, set_id, collector_number)` | 同一逻辑卡、同一 Set 和收藏编号跨语言合并；同 Set 异画或不同收藏编号不得合并 |
| `true` | `true` | `("oracle", oracle_id)` | 同一逻辑卡的全部 Set、收藏编号和语言合并 |

任一记录的 `oracle_id = null` 时，四种开关组合一律使用 `("print", scryfall_id)`，只能与自身分为一组；不得使用空 `oracle_id`、`set_id`、收藏编号或语言把多个无逻辑身份的记录合并。上述键只用于应用筛选后的收藏列表分组和 `CollectionItemDTO` 数量合计，不参与收藏调整；收藏调整始终精确作用于路径 `scryfall_id`。

响应除通用游标字段外包含汇总。`CollectionItemDTO` 字段如下：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| `card` | `CardSummaryDTO` | 否 | 当前分组的展示卡及详情入口 |
| `group_set` | `SetRefDTO` | 是 | `sort=set` 时所属系列；其他排序为 `null` |
| `group_summary` | object | 是 | `sort=set` 时该系列筛选前的权威 `total_quantity` 和按逻辑卡去重的 `distinct_cards`；其他排序为 `null` |
| `normal_quantity` | integer | 否 | 当前合并分组内普通卡总数，最小为 0 |
| `foil_quantity` | integer | 否 | 当前合并分组内闪卡总数，最小为 0 |
| `merged_sets` | boolean | 否 | 当前列表实际生效的 `merge_sets` 请求值；为 `true` 时本项不显示数量加减控件 |
| `merged_languages` | boolean | 否 | 当前列表实际生效的 `merge_languages` 请求值；为 `true` 时本项不显示数量加减控件 |

`CollectionPageDTO` 字段如下：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| `items` | `CollectionItemDTO[]` | 否 | 当前批次；无结果为 `[]` |
| `summary` | object | 否 | 整个收藏的权威页头汇总：`total_quantity` 为实体总数，`distinct_cards` 按逻辑卡去重；不受本次筛选、排序和合并参数影响 |
| `total` | integer | 否 | 应用筛选和合并后的列表项总数，用于结果计数和游标列表控件 |
| `next_cursor` | string | 是 | 后续批次游标；加载完毕为 `null` |

展示卡图和详情目标由后端在当前分组候选中按发行日期从新到旧、同日按内部稳定卡牌标识选择。前端使用返回的 `card.scryfall_id` 进入具体版本详情，并可用只读 `card.oracle_id` 判断逻辑卡身份。`group_set` 和 `group_summary` 在 `sort=set` 时共同提供系列标头信息。列表筛选先作用于收藏明细，再严格按上表对应的分组键求和；合并不会重复或丢失数量。系列标头汇总和页头汇总不随筛选缩小，分别表达该系列和整个收藏的真实持有量。

只有 `merged_sets = false` 且 `merged_languages = false` 的卡位渲染普通／闪卡数量加减控件，并以该项 `card.scryfall_id` 调用 7.1。任一字段为 `true` 时，卡位仍显示合并后的 `normal_quantity` 和 `foil_quantity`，但数量条只读，必须完全隐藏减号、加号及其热区；不得仅以禁用外观保留控件，也不得从合并代表卡的 `scryfall_id` 发起调整。由于 `merged_*` 表示当前有效视图开关，即使某个合并分组当前恰好只有一条明细，也仍按合并卡位处理。

### 7.5 `GET /api/v1/wishlist/options`

结构与收藏选项相同，但只从当前愿望项生成系列和语言选项。

### 7.6 `GET /api/v1/wishlist`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 按关联的 `scryfall_card.name` 模糊查询 |
| `set_id`、`language`、`color`、`mana_value`、`rarity` | string[] | 使用同名重复参数传递；同字段 OR，不同字段 AND |
| `cursor` | string? | 下一批游标 |

排序固定为系列发行日期从新到旧、系列内收藏编号自然升序，不接受 `sort` 和合并参数。每个 `WishlistItemDTO` 包含 `card: CardSummaryDTO`、`group_set: SetRefDTO` 和恒为 `true` 的 `included: boolean`；三个字段均不可为 `null`。页面响应使用通用游标结构。

## 8. 卡组

### 8.1 `GET /api/v1/decks`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 卡组名称不区分大小写模糊查询 |
| `sort` | enum | `updated_desc`（默认）、`created_desc`、`name_asc`、`card_count_desc` |
| `cursor` | string? | 下一批游标 |

列表项：

```json
{
  "deck_id": "12",
  "name": "Superfriends",
  "version": "3",
  "created_at": "2026-10-01T04:00:00Z",
  "updated_at": "2026-10-09T04:00:00Z",
  "cover_card": null,
  "section_counts": {"commander": 1, "mainboard": 99, "sideboard": 0, "maybeboard": 4},
  "counted_total": 100,
  "color_distribution": {"W": 10, "U": 20, "B": 20, "R": 10, "G": 30, "C": 10},
  "exception": {"type": "shortage", "distinct_cards": 2, "missing_quantity": 3}
}
```

`cover_card` 类型为 `CardSummaryDTO | null`；空卡组为 `null`，非空卡组由后端按详情页相同规则选定。`section_counts` 必须包含四个固定分区，空分区为 0。`exception` 类型为 `object | null`：没有异常时为 `null`；存在异常时 `type` 为 `shortage` 或 `deck_usage`，数量不足优先，`distinct_cards` 和 `missing_quantity` 均为非负整数。

### 8.2 `POST /api/v1/decks`

创建空白卡组并写“创建卡组”日志。请求：

```json
{"name": "新卡组"}
```

`name` 必填，并按公共卡组名称边界校验。

响应 `201`，设置 `Location: /api/v1/decks/{deck_id}`：

```json
{"deck_id":"12","name":"新卡组","version":"1"}
```

### 8.3 `GET /api/v1/decks/{deck_id}`

详情和编辑页共用同一响应：

```json
{
  "deck_id": "12",
  "name": "Superfriends",
  "version": "3",
  "source_text": "About\nName Superfriends\n...",
  "created_at": "2026-10-01T04:00:00Z",
  "updated_at": "2026-10-09T04:00:00Z",
  "sections": [
    {
      "type": "mainboard",
      "quantity": 99,
      "cards": [
        {
          "scryfall_id": "00000000-0000-0000-0000-000000000000",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "quantity": 1,
          "type_group": "planeswalker",
          "card": {
            "scryfall_id": "00000000-0000-0000-0000-000000000000",
            "oracle_id": "00000000-0000-0000-0000-000000000010",
            "name": "Nissa, Vital Force",
            "display_name": "生机妮莎",
            "set": {"set_id":"00000000-0000-0000-0000-000000000001","code":"kld","name":"Kaladesh","display_name":"卡拉德许","released_at":"2016-09-30"},
            "collector_number": "163",
            "language": {"code":"en","name":"英文","sort_order":1},
            "released_at": "2016-09-30",
            "cmc": 5,
            "mana_cost": "{3}{G}{G}",
            "colors": ["G"],
            "rarity": "mythic",
            "layout": "normal",
            "images": {"front_url":"https://...","back_url":null,"has_distinct_back":false}
          },
          "translated_type_line": "鹏洛客",
          "translated_oracle_text": "...",
          "owned_quantity": 0,
          "other_decks_quantity": 1,
          "other_decks_count": 1,
          "all_decks_quantity": 2,
          "using_decks": 2,
          "statistics": {
            "cmc": 5,
            "mana_cost": "{3}{G}{G}",
            "colors": ["G"],
            "produced_mana": [],
            "type_groups": ["planeswalker"],
            "oracle_tags": ["Card Advantage"],
            "is_land": false
          },
          "exception": {"type": "shortage", "missing_quantity": 1}
        }
      ]
    }
  ],
  "summary": {
    "counted_total": 100,
    "collection_rate": 0.91,
    "shortage": {"distinct_cards": 2, "missing_quantity": 3},
    "deck_usage": {"distinct_cards": 1, "missing_quantity": 1}
  }
}
```

卡组详情中的具名 DTO 约束：

| DTO | 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- | --- |
| `DeckSectionDTO` | `type` | `deck_section` | 否 | 四个固定分区之一 |
|  | `quantity` | integer | 否 | 本分区 `cards[].quantity` 之和 |
|  | `cards` | `DeckCardDTO[]` | 否 | 空分区为 `[]` |
| `DeckCardDTO` | `scryfall_id` | string | 否 | 与内嵌 `card.scryfall_id` 相同；请求回传该字段 |
|  | `oracle_id` | string | 否 | 与内嵌 `card.oracle_id` 相同；只读，不得放入保存请求 |
|  | `quantity` | integer | 否 | 当前分区编辑数量，最小为 1 |
|  | `type_group` | string | 否 | 后端确定的稳定类型分组代码 |
|  | `card` | `CardSummaryDTO` | 否 | 自动选定的展示版本及详情入口 |
|  | `translated_type_line` | string | 否 | 翻译缺失时回退英文 |
|  | `translated_oracle_text` | string | 是 | 卡牌没有规则叙述时为 `null` |
|  | `owned_quantity` | integer | 否 | 当前逻辑卡的实际拥有量，最小为 0 |
|  | `other_decks_quantity` | integer | 否 | 排除当前 `deck_id` 后，其他卡组计数分区中该逻辑卡的已保存数量总和，最小为 0 |
|  | `other_decks_count` | integer | 否 | 排除当前 `deck_id` 后，在计数分区使用该逻辑卡的其他卡组数，最小为 0 |
|  | `all_decks_quantity` | integer | 否 | 基于当前已保存状态的所有卡组计数分区数量总和，等于 `other_decks_quantity + 当前卡组已保存计数数量` |
|  | `using_decks` | integer | 否 | 基于当前已保存状态使用该逻辑卡的卡组数，等于 `other_decks_count + 当前卡组是否在计数分区使用该牌` |
|  | `statistics` | `DeckCardStatisticsDTO` | 否 | 当前逻辑卡的统计原始属性；四个分区中的每张牌都必须返回 |
|  | `exception` | object | 是 | 无异常为 `null`；否则含 `type` 和非负 `missing_quantity` |
| `DeckCardStatisticsDTO` | `cmc` | number | 否 | 法术力值 |
|  | `mana_cost` | string | 是 | 无费用时为 `null` |
|  | `colors`、`produced_mana`、`type_groups`、`oracle_tags` | string[] | 否 | 无值时为 `[]` |
|  | `is_land` | boolean | 否 | 是否为地牌 |

顶层 `sections` 必须按 `commander`、`mainboard`、`sideboard`、`maybeboard` 顺序返回四项。`summary.counted_total` 统计指挥官、主牌和备牌，备选不计入；`collection_rate` 范围为 0 至 1，`counted_total = 0` 时固定为 0。两个异常汇总对象均包含非负的 `distinct_cards` 和 `missing_quantity`，没有对应异常时仍返回两个零值。统计范围由前端根据外层 `DeckSectionDTO.type` 选择，不能通过删除非统计分区的原始属性来表达；同一逻辑卡同时位于不同分区时，各分区中的 `DeckCardDTO` 独立携带本区数量和同一份统计属性。

- 卡组牌张使用具体展示版本的 `scryfall_id` 发起请求，同时把 `oracle_id` 作为只读属性返回。前端用 `oracle_id` 在本地判断同一逻辑卡、合并同分区数量，用 `scryfall_id` 展示、跳转和保存；后端保存前再次把 `scryfall_id` 解析为 `oracle_id`，不得信任客户端回传逻辑身份。
- 后端按“拥有版本 → 愿望单版本 → 最新印刷版本；同层发行日期降序；同日内部稳定标识”选择用于展示和详情跳转的 `card`。
- 每个 `DeckCardDTO.statistics` 都返回前端计算统计图、概率表和分组所需的原始属性，包括备牌和备选区；前端当前只用指挥官和主牌的 `quantity` 计权。备牌或备选拖入主牌后直接使用随卡牌已有的属性重算，不得为拖拽再次请求后端或 Card DB。
- 各区和各类型列的排序由后端按需求规则确定，前端保持返回顺序。
- `other_decks_quantity` 和 `other_decks_count` 是编辑页的稳定基线。前端按 `oracle_id` 汇总当前未保存编辑态中的指挥官、主牌和备牌数量，备选区不计入，然后计算 `all_decks_quantity = other_decks_quantity + current_edit_quantity`；`using_decks = other_decks_count + 1` 仅在 `current_edit_quantity > 0` 时成立，否则等于 `other_decks_count`。数量加减和分区拖拽只重算上述前端值，不重新请求后端。
- 响应中的 `all_decks_quantity`、`using_decks` 和 `exception` 只描述请求发生时当前卡组的已保存状态，供只读详情页直接展示；编辑页产生未保存修改后必须改用上述基线公式，不得继续把这三个已保存值当作当前编辑结果。
- `GET /api/v1/decks` 的列表项及其 `exception` 不使用上述前端编辑公式，也不增加 `other_decks_quantity` 或 `other_decks_count`。卡组列表始终由后端基于各卡组已保存内容计算最终异常，现有响应结构和异常优先级不变。

### 8.4 `POST /api/v1/decks/{deck_id}/save`

请求提交完整当前编辑态，不提交原始导入文本：

```json
{
  "version": "3",
  "name": "Superfriends upgraded",
  "sections": [
    {
      "type": "commander",
      "cards": [{"scryfall_id": "00000000-0000-0000-0000-000000000000", "quantity": 1}]
    },
    {
      "type": "mainboard",
      "cards": [{"scryfall_id": "00000000-0000-0000-0000-000000000000", "quantity": 2}]
    }
  ]
}
```

- 顶层 `version`、`name`、`sections` 必填；每个分区对象的 `type`、`cards` 必填，每个卡牌对象的 `scryfall_id`、`quantity` 必填。四种分区类型可以不全部出现，缺少的分区按空处理；同一分区内映射到相同 `oracle_id` 的多个 `scryfall_id` 不得形成重复明细，后端按逻辑卡合并数量并校验上限；数量必须为正整数。
- 请求中的 `scryfall_id` 只用于解析和验证对应 `oracle_id`，不会作为 `deck_card` 的持久化字段，也不承诺保存后继续展示该具体印刷。后端验证全部标识仍有效且具有 `oracle_id`，再按当前名称和真实分区内容生成标准 `source_text`；请求体不接受 `oracle_id`。成功响应中的展示 `scryfall_id` 仍由后端按统一展示版本规则重新选择，因此可以与保存请求提交的值不同。这是卡组不保存具体印刷版本的有意产品行为。
- 保存请求中任一 `scryfall_id` 不存在或已经失效时返回 `422 card_reference_invalid`，以 `sections[n].cards[m].scryfall_id` 定位；记录存在但没有 `oracle_id` 时返回 `422 deck_card_not_supported` 并使用相同字段路径。任一失败都不允许部分保存。
- 使用 `version` 乐观锁。同一事务中替换 `deck_card`、更新名称和标准代码、递增版本；规范化后的名称、分区和数量均未变化时不替换明细、不更新 `updated_at`、不递增版本，也不写“修改卡组”日志，直接返回当前完整详情。
- 成功返回更新后的完整卡组详情。
- 版本不匹配返回 `409 deck_version_conflict`，不得覆盖数据库或清空客户端编辑态。

### 8.5 `POST /api/v1/decks/{deck_id}/copies`

无请求体。复制卡组及全部分区，名称以固定前缀 `复制 ` 开始，写一条“复制卡组”日志。名称最长 255 个 Unicode 码点；若 `复制 ` 加原名称超过上限，保留完整前缀并从原名称末尾截断至总长 255，不拆分 Unicode 码点。重复复制继续在当次原名称前增加前缀并应用相同截断规则，不要求名称唯一。

响应 `201`，`Location` 指向新卡组，并返回新卡组的 `deck_id`、名称和版本。

### 8.6 `POST /api/v1/decks/{deck_id}/delete`

请求：

```json
{"version":"3"}
```

服务端必须使用 `deck_id + version` 校验用户确认删除的仍是当前版本。版本不匹配返回 `409 deck_version_conflict`，不得删除已经被其他页面修改的卡组。在同一事务中删除卡组及级联明细并写“删除卡组”日志，日志保留删除前名称快照。

请求字段 `version` 必填。

响应 `200`：`{"deck_id":"12","deleted":true}`。

相同 `Idempotency-Key` 重放时即使卡组已经不存在，也重放首次成功结果；使用新幂等键再次删除则返回 `404 deck_not_found`。

### 8.7 `POST /api/v1/decks/{deck_id}/parse_code`

只解析代码和卡名，不保存卡组、不写操作日志。

请求：

```json
{"source_text":"About\nName Example\n\nDeck\n1 Sol Ring"}
```

`source_text` 必填；空字符串或只含空白返回 `422 validation_failed`。

成功响应 `200`：

```json
{
  "name": "Example",
  "sections": [
    {"type":"commander","cards":[]},
    {
      "type": "mainboard",
      "cards": [
        {
          "scryfall_id": "00000000-0000-0000-0000-000000000000",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "quantity": 1,
          "card": {
            "scryfall_id":"00000000-0000-0000-0000-000000000000",
            "oracle_id":"00000000-0000-0000-0000-000000000010",
            "name":"Sol Ring",
            "display_name":"阳光戒",
            "set":{"set_id":"00000000-0000-0000-0000-000000000001","code":"cmm","name":"Commander Masters","display_name":"指挥官大师","released_at":"2023-08-04"},
            "collector_number":"396",
            "language":{"code":"en","name":"英文","sort_order":1},
            "released_at":"2023-08-04",
            "cmc":1,
            "mana_cost":"{1}",
            "colors":[],
            "rarity":"uncommon",
            "layout":"normal",
            "images":{"front_url":"https://...","back_url":null,"has_distinct_back":false}
          }
        }
      ]
    },
    {"type":"sideboard","cards":[]},
    {"type":"maybeboard","cards":[]}
  ]
}
```

任一行无法识别、名称不唯一或数量非法时，返回 `422 deck_parse_failed`，并一次返回全部已发现问题：

```json
{
  "code": "deck_parse_failed",
  "message": "deck code could not be read",
  "details": [
    {"line": 4, "reason": "card_not_found", "input": "Unknown Card"}
  ]
}
```

失败不得返回部分解析结果。成功时固定返回四个分区且空分区为 `[]`；顺序与输入文本无关，固定为 `commander`、`mainboard`、`sideboard`、`maybeboard`。卡牌行中的 Set 代码、收藏编号和 `*F*` 被有意忽略，不能指定具体印刷、语言或工艺；后端只按数量、标准卡名和分区解析逻辑卡，再按统一规则选择展示印刷。

响应 `name` 类型为 `string | null`。代码中存在合法、非空且未超出长度上限的 `About`／`Name ` 行时返回解析出的规范化名称；没有 `Name ` 行，或 `Name ` 后为空白、超出长度上限时返回 `null`。`null` 表示“未提供有效的名称覆盖”，前端必须保留读取前的当前名称输入值，不得把名称设为 `null`，也不得清空或回退为已保存名称。名称元数据无效不得影响卡牌解析，也不得导致整次解析失败。

解析响应只负责表达卡名解析结果、分区、数量和后端选定的展示印刷，不返回拥有量、其他卡组占用或完整统计属性。前端收到成功结果后，使用响应中的全部 `scryfall_id` 调用 `POST /api/v1/decks/{deck_id}/card_contexts` 批量取得尚未缓存的编辑上下文；上下文请求也成功后才一次性替换当前未保存编辑态。任一请求失败都不得部分覆盖原编辑内容。

### 8.8 `GET /api/v1/decks/{deck_id}/cards/search`

查询参数：`q` 必填、`cursor` 可选。直接按 `scryfall_card.name` 模糊匹配具有非空 `oracle_id`、可加入卡组的具体记录，再由后端按 `oracle_id` 去重，用于当前卡组的增加卡牌弹窗；`deck_id` 不存在时返回 `404 deck_not_found`。

响应使用通用游标结构；每项为 `DeckCardCandidateDTO`：`card` 是后端选定展示版本的 `CardSummaryDTO`，其中包含只读 `oracle_id`；`display_type_line` 是翻译缺失时回退英文的非空字符串，`display_oracle_text` 在没有规则叙述时为 `null`。展示卡图选择规则与卡组详情一致。搜索结果按 `oracle_id` 去重，前端使用返回的只读 `oracle_id` 判断主牌中是否已经存在同一逻辑卡。

候选搜索保持轻量，不返回拥有量、其他卡组占用或完整统计属性。用户首次点击某个尚无编辑上下文的候选项“加入”时，前端先以该项 `card.scryfall_id` 调用下述批量上下文接口；成功后把牌加入本地编辑态，失败时不加入并打开统一错误弹窗。已经缓存上下文的逻辑卡再次加入时不重复请求。

### 8.9 `POST /api/v1/decks/{deck_id}/card_contexts`

这是只读批量查询，不修改卡组、不写操作日志，也不要求 `Idempotency-Key`。请求：

```json
{
  "scryfall_ids": [
    "00000000-0000-0000-0000-000000000000"
  ]
}
```

`scryfall_ids` 是必填字段，必须为去重后的非空数组，最多 200 项。响应保持请求顺序，每项为 `DeckEditCardContextDTO`：

```json
{
  "items": [
    {
      "requested_scryfall_id": "00000000-0000-0000-0000-000000000000",
      "scryfall_id": "00000000-0000-0000-0000-000000000000",
      "oracle_id": "00000000-0000-0000-0000-000000000010",
      "card": {},
      "type_group": "artifact",
      "translated_type_line": "神器",
      "translated_oracle_text": "...",
      "owned_quantity": 1,
      "other_decks_quantity": 2,
      "other_decks_count": 2,
      "statistics": {
        "cmc": 1,
        "mana_cost": "{1}",
        "colors": [],
        "produced_mana": ["C"],
        "type_groups": ["artifact"],
        "oracle_tags": ["Mana Rock"],
        "is_land": false
      }
    }
  ]
}
```

- `requested_scryfall_id` 原样回显请求数组当前位置的标识，只用于前端关联响应；`scryfall_id` 是后端根据解析出的 `oracle_id` 按统一规则重新选择的最终展示版本，两者允许不同。`card` 是完整 `CardSummaryDTO`，其 `scryfall_id`、`oracle_id` 必须与外层最终展示字段一致；卡图选择规则与卡组详情一致。
- 任一请求数组元素不存在或已经失效时，整个请求返回 `422 card_reference_invalid`，并以 `scryfall_ids[n]` 定位原始元素，不返回部分结果。记录存在但 `oracle_id` 为空时返回 `422 deck_card_not_supported`，使用相同字段路径定位。
- `owned_quantity` 是当前逻辑卡实际拥有量；`other_decks_quantity` 和 `other_decks_count` 明确排除当前 `deck_id`，作为前端计算当前未保存编辑态下所有卡组合计及使用卡组数的稳定基线。
- `statistics` 直接复用 `DeckCardStatisticsDTO`；数量不属于该 DTO，前端始终使用卡牌在当前分区的 `quantity` 计权。卡组详情、代码解析后批量补齐上下文和首次增加卡牌三条路径取得的统计属性结构必须一致。
- 前端负责本地维护分区数量、同分区 `oracle_id` 合并、固定排序、总张数、标准代码预览、异常、收集率和统计结果。上下文只在代码解析成功、首次加入尚未缓存的逻辑卡，或缓存被明确失效时批量取得；加减数量、拖拽分区和重复加入已缓存卡牌不得逐次请求后端。
- Card DB 或 App DB 查询失败时返回统一错误信封；前端保留请求前的未保存编辑态。

## 9. 操作日志

App DB 只保存 UTC `occurred_at`，不持久化 `operation_date`。操作日志接口中的 `operation_date` 是后端把 `occurred_at` 转换到 `/app` 返回的 `timezone` 后派生的自然日；每日汇总也按该派生日期和 `action_type` 分组。前端不得根据浏览器本地时区重新划分日期。

所有日期参数都按该应用时区解释。后端必须分别取得本地自然日零点和下一自然日零点，再转换成 UTC，以 `occurred_at >= utc_start AND occurred_at < utc_end` 的左闭右开区间查询。开始日期只提供下界；结束日期使用其下一自然日零点作为排他上界；两端都提供时仍表现为自然日闭区间。不得用固定加减小时数或固定 24 小时推算边界，以保证应用时区存在夏令时变化时仍按真实自然日查询。例如 `Asia/Shanghai` 的 `2026-10-09` 对应 `[2026-10-08T16:00:00Z, 2026-10-09T16:00:00Z)`。

明细响应中的 `occurred_at` 始终保持 UTC RFC 3339，不转换为本地时间字符串；`operation_date` 只用于自然日查询、汇总和页面日期标题。

### 9.1 `GET /api/v1/operation_logs/summary`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `start_date` | date? | 应用时区自然日闭区间开始 |
| `end_date` | date? | 应用时区自然日闭区间结束 |
| `page` | integer | 默认 1，最小 1 |

开始日期晚于结束日期返回 `400 invalid_date_range`。每页固定 10 条，不接受 `page_size`。

响应：

```json
{
  "items": [
    {"operation_date":"2026-10-09","action_type":"add_card","count":3}
  ],
  "page": 1,
  "page_size": 10,
  "total_items": 18,
  "total_pages": 2
}
```

排序固定为日期降序，同日按八种动作的枚举顺序。

### 9.2 `GET /api/v1/operation_logs`

查询参数 `operation_date`、`action_type` 必填，`page` 默认 1。`operation_date` 按应用时区转换为该自然日对应的 UTC 左闭右开区间；每页固定 10 条，按 `occurred_at DESC, id DESC` 排序。

响应项使用 `kind` 判别。由于请求固定为一个 `action_type`，同一响应中的项目只会是 `card` 或 `deck` 一种类型，不会混合：

```json
{
  "items": [
    {
      "operation_id": "91",
      "kind": "card",
      "action_type": "add_card",
      "occurred_at": "2026-10-09T04:00:00Z",
      "card": {
        "name": "Lightning Bolt",
        "set_code": "m11",
        "collector_number": "146",
        "language": "en",
        "finish": "normal",
        "quantity_delta": 1
      }
    }
  ],
  "page": 1,
  "page_size": 10,
  "total_items": 3,
  "total_pages": 1
}
```

愿望单日志的 `finish` 和 `quantity_delta` 为 `null`。卡牌日志的 `language` 返回操作时快照的语言代码，不额外保存或返回语言名称；前端使用与 Set 列表完全相同的一份固定语言代码展示映射，例如 `en → 英文`，未知代码直接显示原代码。卡组动作将 `kind` 设为 `deck`，并以 `deck: {"deck_id":"12","name":"Superfriends"}` 代替 `card`。除语言名称由前端固定映射外，其余展示字段使用日志快照，不依赖当前 Card DB 或卡组仍然存在。

## 10. 卡牌数据维护接口

现有接口保持兼容：

### 10.1 `POST /api/v1/card_data/load`

请求：`{"confirmation":"LOAD_CARD_DATA"}`，其中 `confirmation` 必填。成功返回 `202` 和任务 DTO，并设置任务资源 `Location`。

### 10.2 `GET /api/v1/card_data/load/{task_id}`

成功查询任务始终返回 `200`；即使任务阶段为 `failed`，任务执行错误也放在 `data.error`，不能改用顶层 HTTP 错误。

任务 DTO：

```json
{
  "id": "...",
  "stage": "validating",
  "started_at": "2026-10-09T04:00:00Z",
  "finished_at": null,
  "error": null
}
```

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| `id` | string | 否 | 不透明任务标识 |
| `stage` | enum | 否 | `validating`、`preparing`、`importing`、`verifying`、`switching`、`cleaning`、`completed` 或 `failed` |
| `started_at` | datetime | 否 | 任务启动时间 |
| `finished_at` | datetime | 是 | 终态完成时间；运行中为 `null` |
| `error` | object | 是 | 失败详情；非失败状态为 `null` |

`error` 非空时必须包含 `stage` 和面向运维但不泄露 SQL、路径或堆栈的 `message`；只有源文件错误才返回相对文件名 `file`，只有可定位到输入行时才返回从 1 开始的 `line`。`verifying` 同时校验临时表关联完整性与活动个人数据引用；引用缺失会使任务失败并阻止换表。

## 11. 状态码与错误代码

| HTTP | `error.code` | 场景 |
| ---: | --- | --- |
| 400 | `invalid_request` | JSON、参数类型、未知字段或参数组合错误 |
| 400 | `invalid_cursor` | 游标无效、过期或与当前查询条件不匹配 |
| 400 | `invalid_date_range` | 开始日期晚于结束日期 |
| 400 | `confirmation_required` | 卡牌加载确认文本错误 |
| 400 | `idempotency_key_required` | 状态变更接口缺少或未正确填写 `Idempotency-Key` |
| 403 | `load_disabled` | 卡牌数据加载未启用 |
| 404 | `not_found` | 路由不存在 |
| 404 | `set_not_found` | Set 不存在 |
| 404 | `card_not_found` | 路径参数中的 `scryfall_id` 不存在或已失效；不用于请求体字段校验 |
| 404 | `deck_not_found` | 卡组不存在 |
| 404 | `task_not_found` | 加载任务不存在 |
| 409 | `load_in_progress` | 已有加载任务运行 |
| 409 | `collection_underflow` | 收藏减少后将小于零 |
| 409 | `deck_version_conflict` | 卡组乐观锁版本不匹配 |
| 409 | `idempotency_conflict` | 同一幂等键已用于不同方法、路径或请求体 |
| 405 | `method_not_allowed` | 路径存在但 HTTP 方法不受支持 |
| 413 | `request_too_large` | 请求体超过服务端限制 |
| 415 | `unsupported_media_type` | 带请求体的接口未使用 `application/json` |
| 422 | `validation_failed` | 字段值或业务规则不合法 |
| 422 | `card_reference_invalid` | 保存卡组或查询编辑上下文时，请求体中的 `scryfall_id` 不存在或已失效；`details[].field` 定位具体元素 |
| 422 | `finish_not_available` | 当前印刷不支持请求的工艺 |
| 422 | `deck_parse_failed` | 卡组代码存在无法识别或歧义行 |
| 422 | `deck_card_not_supported` | 请求的 `scryfall_id` 没有逻辑卡牌身份，不能加入卡组 |
| 500 | `internal_error` | 未预期服务端错误 |
| 503 | `dependency_unavailable` | Card DB 或 App DB 暂不可用 |

全局卡牌搜索无法完整识别语法时不是错误，必须回退名称查询并返回 `200`。合法查询零结果、空收藏、空愿望单、空卡组和空日志同样返回 `200`。

## 12. 事务与一致性

- 收藏数量变化及其日志必须在同一 App DB 事务中提交。
- 愿望单状态变化及其日志必须在同一事务中提交；重复的幂等请求不产生日志。
- 卡组创建、复制、修改、删除及其日志必须分别在单一事务中提交。
- 卡组保存必须先完成全部 Card DB 标识验证，再开始 App DB 写事务，不允许部分保存。
- Card DB 与 App DB 不执行跨库事务或跨库外键；Card DB 读取失败时不得继续产生依赖该读取的个人数据写入。
- 个人数据 mutation 的 Card DB 验证与随后 App DB 提交必须处于同一共享维护门闩内；全量卡牌数据任务在活动引用校验至换表结束期间持有独占门闩，避免验证后出现悬空引用。
- 列表 mutation 成功后返回当前资源的权威局部状态，前端据此更新当前卡位，不要求重新请求整个列表。

## 13. OpenAPI 契约交付要求

本文评审通过后转换为 OpenAPI 3.1：

- 每个请求和响应使用具名 schema，不直接以内联匿名对象代替核心 DTO。
- 所有响应引用统一 `Envelope`、`ErrorDetail` 和分页 schema。
- 为数组参数声明 `style: form`、`explode: true`。
- 为字符串枚举、UUID、日期、时间、整数范围和 nullable 字段声明准确约束。

## 14. 需求覆盖

| 需求章节 | 对应接口 |
| --- | --- |
| 1 页面身份标识 | `GET /app` |
| 2 卡牌查看与检索 | `GET /sets/options`、`GET /sets`、`GET /cards`、`GET /search` |
| 3 卡牌详情 | `GET /cards/{scryfall_id}` |
| 4 我的卡牌 | `GET /collection/options`、`GET /collection`、`POST /cards/{scryfall_id}/collection_adjustments` |
| 5 愿望单 | `GET /wishlist/options`、`GET /wishlist`、`POST /cards/{scryfall_id}/wishlist_state` |
| 6 卡牌中心复合 | 复用卡牌列表、收藏、愿望单和详情接口 |
| 7 卡组列表 | `GET/POST /decks`、`POST /decks/{deck_id}/copies` |
| 8 卡组编辑 | `GET /decks/{deck_id}`、`POST /decks/{deck_id}/save`、`POST /decks/{deck_id}/delete`、`POST /decks/{deck_id}/parse_code`、`GET /decks/{deck_id}/cards/search`、`POST /decks/{deck_id}/card_contexts` |
| 9–13 卡组展示版本、拥有量、异常、使用情况与统计 | `GET /decks/{deck_id}`、`GET /cards/{scryfall_id}` |
| 14 操作日志 | `GET /operation_logs/summary`、`GET /operation_logs` |
| 15 加载卡牌数据 | `POST /card_data/load`、`GET /card_data/load/{task_id}` |

表中省略的路径均以 `/api/v1` 为前缀。
