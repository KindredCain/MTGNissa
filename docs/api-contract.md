# MTGNissa API 契约

## 1. 文档状态

- 状态：首版业务 API 契约评审稿
- 契约版本：`v1`
- 基础路径：`/api/v1`
- 数据格式：`application/json; charset=utf-8`
- 需求基线：[功能需求](requirements.md)、[搜索语法支持矩阵](search-syntax-support.md)、[新数据字典](card-data-dictionary.md)与 [原型说明](../design/README.md)；本轮已按新字典及已确认原型规则对齐。
- 边界：本文定义 HTTP 路径、请求响应 DTO、分页、并发和错误代码；业务计算及页面交互仍以功能需求和 `design/` 为准。

本期不实现认证。业务 API 与维护接口均应只部署在可信网络或受访问控制的反向代理之后。

## 2. 通用约定

### 2.1 响应信封

所有响应，包括成功、校验失败、未找到路由和服务端异常，都使用统一信封。HTTP 状态码必须与 `status` 相同。

成功：

```json
{
  "status": 200,
  "data": {"deck_id": "12", "name": "Superfriends"},
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

`ErrorDTO` 必须包含非空 `code` 和安全的 `message`。可选 `details` 是对象数组；字段校验项使用 `field` 与 `reason`，代码解析项使用从 1 开始的 `line`、`reason` 和可选的安全输入摘要 `input`。同一项不得同时返回 `field` 和 `line`。`field` 使用从请求 DTO 根开始的点号与零基数组下标格式，例如 `sections[1].cards[3].printing_uuid`、`printing_uuids[2]`；不得混用 JSON Pointer、美元符号路径或一基数组下标。

下文各接口的响应示例为统一信封中的 `data` 内容，实际 HTTP 响应仍必须包裹完整信封。

### 2.2 请求约定

- JSON 字段、路径参数、查询参数以及前端 API 成员名统一使用 `snake_case`；例如路由动作 `parse_code` 在前端同名表示为 `parse_code`。
- Go 导出类型、字段和处理函数统一使用 `UpperCamelCase`，按单词边界映射上述名称，例如 `parse_code` 对应 `ParseCode`，`printing_uuid` 对应 `PrintingUUID`。
- JSON 请求对象拒绝未知字段、尾随的第二个 JSON 值和错误类型。
- 除非端点正文明确标记为可选，请求示例和请求字段表中的字段均为必填。缺少必填字段返回 `422 validation_failed` 并在 `error.details[].field` 中定位；未知字段、错误 JSON 类型或多余 JSON 值返回 `400 invalid_request`。
- 查询参数同样拒绝未在当前端点声明的名称；例如擅自传入 `limit` 返回 `400 invalid_request`，不得静默忽略。可选查询参数会在各端点明确标记，类型后的 `?` 表示可省略；可选多选筛选参数省略时表示不限制该维度，排序及合并参数省略时使用端点声明的默认值。重复参数只允许用于声明为数组的字段。
- 客户端可发送不超过 128 字符的 `X-Request-ID`；服务端始终在响应头和信封中返回最终请求 ID。
- 时间戳在数据库中统一以 UTC 保存，并在接口中使用 RFC 3339 UTC 表示，例如 `2026-10-09T04:00:00Z`；自然日使用 `YYYY-MM-DD`，其时区语义由具体端点声明。
- 业务资源标识作为不透明字符串返回；前端只用于回传、路由和缓存键，不解析其来源或结构。
- MySQL `BIGINT UNSIGNED` 标识统一按十进制字符串返回，避免 JavaScript 精度丢失。
- 多选查询参数使用重复参数，例如 `language=en&language=ja`，不使用逗号拼接。
- 查询关键字去除首尾空白，空白按未提供处理；所有名称查询均不区分大小写，并将用户输入的 `%`、`_` 及转义字符作为普通文字处理。全局名称搜索及收藏、愿望单名称筛选匹配 `mtgnissa_card.name`；普通 Set 卡牌列表与卡组增加卡牌的正式名称搜索匹配 `mtgnissa_card_printing.printed_name`，执行文字包含匹配 `%q%`；卡组名称联想匹配同一 `printed_name` 字段，执行文字前缀匹配 `q%`，最多 5 个去重名称。Token 列表不接受名称、语言、筛选或排序参数。各端点再应用自己的范围、去重和分页规则，不得将全局名称搜索规则套用于所有列表。
- 只有收藏调整、愿望单状态、创建卡组和复制卡组四类操作必须携带 `Idempotency-Key` 请求头。客户端为一次用户意图生成一个 RFC 4122 UUID，并使用 36 字符规范文本形式；同一次请求发生超时或连接中断时复用原值，新的用户操作必须生成新值。保存卡组、删除卡组、只读 `GET`、只读的卡组编辑上下文批量查询、卡组代码解析和卡牌数据维护任务不使用该键。
- 幂等只保证同一业务意图在当前单实例服务进程和缓存有效期内不被重复执行，不保证逐字重放首次 HTTP 响应。服务端在有界内存缓存中保存幂等键、请求指纹、`executing`／`completed` 状态、创建或复制操作产生的新 `deck_id` 等必要资源标识及过期时间；不得为此在 App DB 建表，也不要求缓存完整响应信封、HTTP 状态、`data` 或 `Location`。完成记录保留 30 分钟；服务进程重启后记录允许丢失，首版不提供跨实例幂等保证。
- 请求指纹由方法、含实际资源标识的规范化路径和解码后 DTO 的规范 JSON 计算，JSON 空白和对象字段顺序不影响指纹。相同键对应不同请求时返回 `409 idempotency_conflict`。两个相同请求并发到达时只能有一个执行 mutation；另一个等待首次请求结束。
- 首次请求成功提交后将内存记录标记为 `completed`。缓存有效期内的相同请求不得再次改变状态或重复写日志，而是重新查询该操作对应资源的当前权威状态并返回；响应可以反映首次提交之后发生的其他合法变化，不要求与首次响应逐字一致，并增加响应头 `Idempotency-Deduplicated: true`。创建或复制卡组的重复请求使用记录中的新 `deck_id` 查询该卡组并重新生成 `Location`；如果该卡组后来已被删除，则返回 `404 deck_not_found`，不得再次创建或复制。
- 请求解析、字段校验、依赖故障、内部错误以及未成功提交的业务结果不保留为 `completed`；等待中的相同请求收到同一次失败，之后允许使用同一键重新发起。客户端修正请求内容后必须使用新的幂等键。

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

普通 Set 卡牌列表一次返回当前 Set、语言和名称关键字下的完整轻量结果，Token Set 一次返回完整轻量印刷项；两者不使用服务端游标，卡图按前端本地批次渲染。名称联想只返回最多 5 项，不分页。Set 索引的游标仅对顶层父系列分页，具体分组规则见 5.2。操作日志使用固定每页 10 条的页码分页，不使用游标。

### 2.4 公共枚举

```text
finish_kind: normal | foil
deck_section: commander | mainboard | sideboard | maybeboard
deck_card_type_group: land | creature | planeswalker | instant | sorcery | enchantment | artifact | other
operation_type:
  add_card | remove_card | add_wishlist | remove_wishlist |
  create_deck | delete_deck | copy_deck | modify_deck
color_group: W | U | B | R | G | multicolor | colorless
mana_value_group: 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8_plus
rarity: common | uncommon | rare | mythic | special | bonus
```

公开收藏工艺枚举有意只保留 `normal` 与 `foil`，分别表示同一具体卡牌记录上的平卡和闪卡两个独立收藏计数桶。每个 `printing_uuid` 均允许维护这两个计数，不根据 Card DB 中该印刷的原始工艺属性限制收藏调整。需要映射外部工艺值时，普通工艺归入 `normal`，传统闪和蚀刻闪统一归入 `foil`；收藏数量和日志不再区分传统闪与蚀刻闪。

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
  "code": "KHM",
  "name": "Kaldheim",
  "display_name": "凯德海姆",
  "released_at": "2021-02-05"
}
```

`CardSummaryDTO` 是卡牌列表、搜索结果和卡组卡图共同使用的轻量展示对象：

```json
{
  "printing_uuid": "00000000-0000-0000-0000-000000000000",
  "oracle_id": "00000000-0000-0000-0000-000000000010",
  "name": "Nissa, Vital Force",
  "display_name": "生机妮莎",
  "set": {
    "code": "KLD",
    "name": "Kaladesh",
    "display_name": "卡拉德许",
    "released_at": "2016-09-30"
  },
  "collector_number": "163",
  "language": {"code": "ja", "display_name": "日文", "sort_order": 7},
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

`printing_uuid` 是 `mtgnissa_card_printing.uuid`，用于定位普通卡牌的具体印刷及语言版本，也是详情、收藏、愿望单、卡组候选上下文、编辑提交和日志使用的业务标识。单面牌使用自身语言印刷行的 UUID；双面或多牌面牌的各卡面行各有 UUID，对外版本入口统一使用同一印刷、同一语言主牌面（`face_index = 0`）的语言印刷 UUID，其余牌面随同返回，不拆分收藏或愿望单。客户端只回传后端返回的规范 `printing_uuid`，不得改用基础卡面的 `mtgnissa_card.uuid` 或图片 ID。

`scryfall_id` 是可空的外部图片来源标识，不作为普通卡牌请求参数或业务键。后端通过语言印刷行的 `card_uuid` 读取基础卡面；图片优先使用语言印刷行的 `scryfall_id`，缺失时仅图片来源回退到 `mtgnissa_card.scryfall_id`，并返回已经解析完成的 `CardImageDTO`。该回退不改写 `printing_uuid`、语言、收藏、愿望单、跳转目标或日志身份。前端只消费图片 URL，不自行拼接地址。Token 另用 `TokenSummaryDTO`，不复用普通卡牌业务入口。

`oracle_id` 是只读逻辑卡牌标识，数据源缺失时为 `null`。前端可用它在本地去重、合并和判断同卡，但不得放入路径、查询参数或 mutation 请求；请求仍提交 `printing_uuid`，后端解析其 `oracle_id` 后执行逻辑卡查询、统计和卡组持久化。无 Oracle 身份的卡不允许加入卡组。`deck_card` 的数据库身份为 `(deck_id, section_type, oracle_id)`，API 不定义 `deck_card_id`。Set 业务标识为 `mtgnissa_set.code`；查询参数称 `set_code`，`SetRefDTO` 及 Set 列表项中直接用 `code` 表示，不再提供独立 Set UUID。

收藏和愿望单的具体版本引用以主牌面 `printing_uuid` 为键；同一印刷、同一语言的多个牌面共用数量或星标，不将共享卡图或缺失外部图片 ID 的不同语言版本合并。App DB 只按该业务身份及所需快照处理个人状态；Card DB 展示信息按当前批次 UUID 集合批量补齐，不执行跨数据库 SQL JOIN。稳定分组键、详情目标和 mutation 目标均使用 `printing_uuid`，图片 ID 仅供图片解析及既定非语言基础印刷选择使用。

公共 DTO 的完整字段约束：

| DTO | 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- | --- |
| `LanguageDTO` | `code` | string | 否 | 语言代码 |
|  | `display_name` | string | 否 | 直接取 `scryfall_language.display_name`；前端原样展示 |
|  | `sort_order` | integer | 否 | 字典顺序 |
| `CardImageDTO` | `front_url` | string | 是 | 数据源没有卡图时为 `null`，客户端使用统一占位图 |
|  | `back_url` | string | 是 | 没有独立背面时为 `null` |
|  | `has_distinct_back` | boolean | 否 | 是否显示翻面控件 |
| `SetRefDTO` | `code` | string | 否 | `mtgnissa_set.code`，也是唯一系列业务标识 |
|  | `name` | string | 否 | 英文名称 |
|  | `display_name` | string | 否 | 翻译后名称，缺失时为英文名称 |
|  | `released_at` | date | 是 | 数据源缺失时为 `null` |
| `CardSummaryDTO` | `printing_uuid` | string | 否 | 主牌面语言印刷 UUID，具体印刷及语言版本业务标识，也是详情入口 |
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

`CardSummaryDTO` 的 `cmc`、`mana_cost` 和 `colors` 使用当前具体印刷的主牌面记录：普通单面牌取 `face_index = -1`，多牌面牌取 `face_index = 0`。基础卡面的多个牌面可能共享 `mtgnissa_card.scryfall_id`，必须按 `face_index` 组织，不能随机选行或合并这些字段。该规则只约束摘要、筛选和排序使用的单组属性，不改变全局搜索在逻辑卡层“任一牌面满足单个条件”的语义。翻译通过当前 `mtgnissa_card.uuid -> card_translation.card_id`、`card_oracle_translation_mapping -> oracle_translation` 和风味翻译映射取得，逐字段沿用既有优先级，中文缺失时回退对应英文；Oracle 标签经 `oracle_tag_mapping` 关联，不混合多个牌面的翻译。

### 2.6 输入边界

| 输入 | 约束 |
| --- | --- |
| `printing_uuid`、`cover_printing_uuid`、`printing_uuids[]` | 规范 UUID 字符串；使用服务端返回的语言印刷业务标识，多牌面使用主牌面 UUID |
| `set_code` | 非空字符串，最长 16 字符；使用后端返回的 `mtgnissa_set.code` 原值，不是 UUID，客户端不改变大小写 |
| 响应中的 `oracle_id` | 规范 UUID 字符串或 `null`；只读，不能作为请求参数 |
| `deck_id` | 十进制正整数字符串，最长 20 位 |
| 列表和搜索 `q` | 去除首尾空白后最长 512 个 Unicode 字符；全空白按未提供处理，全局搜索的非空 `q` 必填 |
| 卡组名称 | 去除首尾空白后 1–255 个 Unicode 字符；保存规范化后的值 |
| 卡组代码 | UTF-8 文本，最长 768 KiB、最多 1,000 行 |
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
| `GET` | `/api/v1/sets/{set_code}/tokens` | 当前 Token Set 的完整轻量印刷列表，不带语言、搜索、筛选或排序参数 |
| `GET` | `/api/v1/cards/{printing_uuid}` | 卡牌详情 |
| `POST` | `/api/v1/cards/{printing_uuid}/collection_adjustments` | 调整普通或闪卡数量 |
| `POST` | `/api/v1/cards/{printing_uuid}/wishlist_state` | 设置愿望单状态 |
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
| `GET` | `/api/v1/decks/{deck_id}/cards/name_suggestions` | 增加卡牌名称前缀联想，最多 5 个名称 |
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

返回 Set 列表使用的动态筛选项 `SetOptionsDTO`：

```json
{
  "languages": [
    {"code": "en", "display_name": "英文", "sort_order": 1}
  ],
  "set_types": ["commander", "core", "expansion"]
}
```

`languages` 从 `scryfall_language` 按 `sort_order ASC, code ASC` 只返回前 11 项。`set_types` 只取 `mtgnissa_set.parent_code IS NULL` 的顶层父 Set 中非空去重类型；仅出现在子系列或 Token Set 的类型不进入选项。按产生该值的最小父 Set `code` 升序、`set_type` 升序兜底。

### 5.2 `GET /api/v1/sets`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 只匹配顶层父 Set 的系列名称或代码，不区分大小写的包含匹配 |
| `language` | string[]? | 可省略，省略表示全部语言；同一字段 OR；只判断顶层父 Set 的 `languages` 是否包含至少一种所选语言 |
| `set_type` | string[]? | 可省略，省略表示全部类型；只判断顶层父 Set 类型；同一字段 OR，值域来自 5.1 |
| `sort` | enum? | 可省略，默认 `released_desc`；可选值为 `released_desc`、`released_asc`、`name_asc`、`name_desc`、`code_asc`、`code_desc`、`card_count_desc`、`card_count_asc` |
| `cursor` | string? | 下一批游标 |

`language` 与 `set_type` 之间使用 AND。

查询、筛选、排序和游标分页只作用于 `parent_code IS NULL` 的顶层父 Set；父 Set 主排序相同时按 `code ASC` 兜底，游标包含最终父节点排序位置。父节点命中后，其完整子树不论是否符合当前条件都随父节点返回并保持缩进。

同一父节点下先普通子 Set、后 Token Set，两组内固定按 `released_at ASC, code ASC`，递归以前序遍历展开。发行日缺失时在各升序组内排在有日期记录之后，再按 `code ASC`；父节点日期排序同样将缺失日期排在有日期之后。每页按父组数量截取，不得在子树内部截断，子孙行不消耗父组分页名额。响应使用 `SetPageDTO`：`items: SetListItemDTO[]` 为当前父组及完整子树的平面前序数组；`total` 是条件命中的顶层父组总数；`next_cursor` 是下一父组游标或 `null`。可见行数可以大于每批父组数，前端不得将其当作分页已完成的依据。

列表项：

```json
{
  "code": "KHM",
  "name": "Kaldheim",
  "display_name": "凯德海姆",
  "set_type": "expansion",
  "card_count": 405,
  "released_at": "2021-02-05",
  "languages": [
    {"code": "en", "display_name": "英文", "sort_order": 1, "available": true}
  ],
  "default_language": "en",
  "parent_code": null,
  "depth": 0
}
```

`parent_code` 直接取 `mtgnissa_set.parent_code`，根节点为 `null`；`depth` 为从 0 开始的层级。后端按权威父代码构树，不根据名称、日期或类型猜测关系；缺失父节点或循环属于数据错误，返回 `503 dependency_unavailable`，不静默平铺。

`SetListItemDTO` 的完整字段为示例列出的全部键，`code`、`name`、`display_name`、`set_type`、`card_count`、`languages`、`depth` 不可空；`released_at`、`default_language`、`parent_code` 可空，数量和层级为非负整数。普通 Set 的 `languages` 为固定前 11 种语言及其 `available: boolean`，可用性读取本行 `mtgnissa_set.languages`；`default_language` 是从左到右第一项 `available = true` 的代码，无可用语言时为 `null`，该普通行不可跳转。Token Set 固定 `set_type = token`、`languages = []`、`default_language = null`；Token 行始终可直接进入 5.5，不携带语言参数。

`released_at`、`card_count` 均直接读取 `mtgnissa_set` 权威列。`card_count` 是导入纳入的卡面数：普通系列取源 `cards[]` 计数，Token 系列取源 `tokens[]` 计数，不按收藏编号或语言版本重新聚合；业务请求期间不重算。系列译名按 `set_translation.code = mtgnissa_set.code` 取得，缺失时回退英文；Token 可按 `parent_code` 复用来源系列译名。列表图块总数按组织后的印刷项计数，可与 Set 索引的卡面数不同。

### 5.3 `GET /api/v1/cards`

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `set_code` | 是 | 当前普通 Set 的 `mtgnissa_set.code` |
| `language` | 是 | 当前卡牌原语言代码 |
| `q` | 否 | 当前 Set、当前语言内按 `mtgnissa_card_printing.printed_name` 文字包含匹配，不解析语法 |

响应 `SetCardsDTO` 一次返回完整轻量结果，字段为 `set: SetRefDTO`、`language: LanguageDTO`、`items: CardSummaryDTO[]` 和非负整数 `total`，均必填且不可空：

```json
{
  "set": {
    "code": "KLD",
    "name": "Kaladesh",
    "display_name": "卡拉德许",
    "released_at": "2016-09-30"
  },
  "language": {"code": "en", "display_name": "英文", "sort_order": 1},
  "items": [],
  "total": 0
}
```

后端按 `mtgnissa_card_printing -> mtgnissa_card -> 翻译` 读取当前 Set、语言和名称条件内的完整轻量结果；只返回普通卡牌，双面或多牌面按具体印刷及语言组织为一个 `CardSummaryDTO`，统一使用主牌面 `printing_uuid`。当前语言属于项目字典但该 Set 无记录时返回空结果，未知语言代码返回 `422 validation_failed`；Set 不存在返回 `404 set_not_found`。Token Set 不接受本接口，返回 `400 invalid_request`，客户端应使用 5.5。

前端在完整结果上执行颜色、法术力、稀有度筛选和收藏编号／法术力／名称排序，再本地分批显示图片；主排序相同时按收藏编号自然升序、`printing_uuid ASC` 兜底。`total` 为本次名称条件内组织后的印刷列表项数，不是卡面数；前端再计算本地筛选后的计数。

### 5.4 `GET /api/v1/cards/{printing_uuid}`

返回当前 `printing_uuid` 指向的具体印刷及语言版本详情。`printing_uuid` 来自任一列表、搜索、卡组或版本选项响应：

```json
{
  "card": {
    "printing_uuid": "00000000-0000-0000-0000-000000000000",
    "oracle_id": "00000000-0000-0000-0000-000000000010",
    "name": "Nissa, Vital Force",
    "display_name": "生机妮莎",
    "set": {
      "code": "KLD",
      "name": "Kaladesh",
      "display_name": "卡拉德许",
      "released_at": "2016-09-30"
    },
    "collector_number": "163",
    "language": {"code": "ja", "display_name": "日文", "sort_order": 7},
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
      "face_index": -1,
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
  "rulings": [
    {"published_at": "2016-09-20", "text": "..."}
  ],
  "oracle_tags": [
    {"tag_id": "...", "query": "otag:\"Ramp\""}
  ],
  "languages": [
    {
      "printing_uuid": "00000000-0000-0000-0000-000000000002",
      "oracle_id": "00000000-0000-0000-0000-000000000010",
      "language": {"code": "en", "display_name": "英文", "sort_order": 1},
      "normal_quantity": 0,
      "foil_quantity": 0
    }
  ],
  "other_versions": [
    {
      "set": {"code": "CMM", "name": "Commander Masters", "display_name": "指挥官大师", "released_at": "2023-08-04"},
      "collector_number": "307",
      "released_at": "2023-08-04",
      "normal_quantity": 0,
      "foil_quantity": 0,
      "languages": [
        {
          "printing_uuid": "00000000-0000-0000-0000-000000000003",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "language": {"code": "en", "display_name": "英文", "sort_order": 1},
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
| `face_index` | integer | 否 | 保留基础卡面顺序：普通单面及独立牌为 `-1`，多牌面为 `0/1/2/3/4`，按源卡面顺序升序，不重新编号 |
| `name`、`display_name` | string | 否 | 原名及按语言回退后的展示名 |
| `mana_cost` | string | 是 | 该牌面没有费用时为 `null` |
| `type_line`、`display_type_line` | string | 是 | 数据源缺失时为 `null` |
| `oracle_text`、`display_oracle_text` | string | 是 | 没有规则叙述时为 `null` |
| `flavor_text`、`display_flavor_text` | string | 是 | 没有背景叙述时为 `null` |
| `power`、`toughness`、`loyalty` | string | 是 | 不适用或缺失时为 `null`，保留原始符号文本 |
| `image_url` | string | 是 | 当前牌面没有独立图片时为 `null` |

详情按路径 `printing_uuid` 读取语言印刷行，通过 `card_uuid` 关联基础卡面、翻译及 Oracle 数据；路径只接受后端返回的版本入口 UUID，不能用基础卡面 UUID 或图片 ID 替代。对该行所属印刷按基础卡面关系组织同语言的全部牌面，顶层 `card.printing_uuid` 为主牌面版本入口；`faces` 至少一项且按 `face_index` 排序。多牌面共用同一份收藏与愿望单。`collection` 表示该具体版本的普通、闪卡数量；`logical_collection` 表示同一 `oracle_id` 下全部版本合计，没有 `oracle_id` 时只汇总当前 `printing_uuid`。数量均非负；`wishlist.included` 为 boolean。`rulings`、`oracle_tags`、`languages`、`other_versions` 和 `deck_usage.decks` 无结果时为 `[]`；没有 Oracle 身份时相关 Oracle 标签、释疑及卡组使用为空，不把多个空 Oracle 对象关联在一起。

- `languages` 返回当前系列和收藏编号下的实际语言选项；每项包含可直接进入详情的 `printing_uuid`、语言和当前拥有数量。顶层 `languages` 及每个 `other_versions[].languages` 均按 `language.sort_order` 升序，`sort_order` 相同时按 `language.code` 升序；不应用 Set 列表“只取前 11 项”的范围限制。
- `other_versions` 以 `set_code + collector_number` 为分组键，排除当前正在查看的版本分组，不按语言重复版本行。每项返回系列、收藏编号、该版本全部语言的拥有数量合计，以及实际存在的完整 `languages` 数组；语言项携带可直接进入详情的 `printing_uuid`。版本按发行日期从新到旧排序，发布日期相同时按 Set 缩写、收藏编号和该版本分组 `languages[].printing_uuid` 中的最小值升序兜底。
- `rulings`、`oracle_tags` 和 `deck_usage` 随详情一次返回，不另设分页接口。
- `rulings` 每项固定为 `published_at: date` 与 `text: string`，均非空，不返回 `source`。读取 `oracle_ruling` 并按 `ruling_key` 可选关联 `ruling_translation`，正文优先非空 `translation`，否则用 `oracle_ruling.text`。按 `published_at DESC, ruling_key ASC` 返回，前端保持该顺序。
- `oracle_tags` 固定按 `tag_id` 升序返回；前端直接保持该顺序，不按 `query` 重新排序。
- `deck_usage.decks` 固定按卡组 `updated_at` 从新到旧返回，更新时间相同时按 `deck_id` 升序兜底；排序使用后端卡组记录的更新时间，不要求为了排序向响应额外暴露 `updated_at`。
- `oracle_tags[]` 的每项固定包含不可空的 `tag_id` 和 `query`，不再另行返回与查询语法重复的展示字段。`query` 由后端生成为可直接提交给全局搜索的规范查询，固定格式为 `otag:"<escaped label>"`；标签中的反斜杠先转义为 `\\`，双引号转义为 `\"`，即使标签不含空格也保留双引号。该字符串同时是前端的展示值和点击查询值：前端必须原样显示，并在点击时将同一值作为 `GET /api/v1/search` 的 `q` 参数提交，不得自行拼接、去引号、规范化或再次转义。
- 翻译字段已经由后端按“简体中文优先，缺失回退英文”生成；前端不再实现语言选择逻辑。

### 5.5 `GET /api/v1/sets/{set_code}/tokens`

路径 `set_code` 为 Token Set 的系列代码。本接口不接受任何查询参数，包括 `q`、`language`、筛选、`sort`、`cursor` 和 `limit`；传入返回 `400 invalid_request`。Set 不存在返回 `404 set_not_found`；存在但不是 `set_type = token` 时返回 `400 invalid_request`。只读查询，无个人数据或 mutation 入口。

响应为 `TokenSetCardsDTO`，完整字段如下：

```json
{
  "set": {"code": "TKLD", "name": "Kaladesh Tokens", "display_name": "卡拉德许衍生物", "released_at": "2016-09-30"},
  "items": [
    {
      "token_uuid": "00000000-0000-0000-0000-000000000020",
      "name": "Beast",
      "collector_number": "1",
      "layout": "token",
      "images": {"front_url": "https://cards.scryfall.io/normal/front/...jpg", "back_url": null, "has_distinct_back": false}
    }
  ],
  "total": 1
}
```

`set: SetRefDTO`、`items: TokenSummaryDTO[]`、`total: integer` 均必填且非空值，零结果为 `items = []`、`total = 0`。`TokenSummaryDTO` 固定包含 `token_uuid`、`name`、`collector_number`、`layout`、`images`，前四项为非空字符串，`images` 为 `CardImageDTO`；没有 `printing_uuid`、语言、收藏、愿望单、法术力或稀有度字段，不得按 `CardSummaryDTO` 解码。

直接读取 `mtgnissa_token.set_code` 范围内全部卡面。同一印刷共享非空 `scryfall_id` 的多牌面按 `face_index` 组织为一个列表项，`token_uuid` 取主牌面 `mtgnissa_token.uuid`；不按 `oracle_id` 合并不同印刷。空公开 ID 且无法可靠配对的记录按各自 `uuid` 保留并返回缺图占位信息。按主牌面收藏编号自然升序、主牌面 `uuid ASC` 固定返回。`total` 是组织后的印刷项数量，不是 Set 索引的导入卡面数；双面 Token 只占一个卡位，按 `images` 能力原位翻面，不提供普通卡牌详情跳转。

一次返回完整轻量结果，前端按本地批次显示图片，不请求后端翻页。图片缺失时 `front_url = null`；Token 不进入语言印刷表，也不套用普通卡牌的图片回退链路。

## 6. 全局搜索

### 6.1 `GET /api/v1/search`

该接口独立承载全局名称查询、完整语法解析、条件组合、去重、稳定排序和游标分页，不属于普通卡牌列表接口。

查询参数：`q` 必填，`cursor` 可选。

响应：

```json
{
  "input": "function:tutor",
  "mode": "syntax",
  "executed_query": "otag:\"tutor\"",
  "items": [],
  "total": 0,
  "next_cursor": null
}
```

- `mode` 为 `syntax` 或 `name`。
- 只有完整识别首版语法时才使用 `syntax`；否则将完整输入按 `mtgnissa_card.name` 模糊查询并返回 `name`，不得返回语法错误。
- 名称模式先在具体卡牌记录上应用名称条件，只保留具有非空 `oracle_id` 的命中记录，再按 `oracle_id` 去重。语法模式必须先为每个原子条件分别计算满足条件的 `oracle_id` 集合：名称、类型、关键字异能、费用和数值等牌面属性只要同一逻辑卡任一牌面满足该原子条件，该 `oracle_id` 就进入对应集合；系列、稀有度等印刷属性只要任一具体印刷满足该原子条件，该 `oracle_id` 就进入对应集合。隐式 `AND`、`OR` 和括号组合必须在 `oracle_id` 集合层执行，不得要求同一牌面或同一具体印刷同时满足完整表达式。取得最终 `oracle_id` 集合后，后端再为每项选择代表记录，按逻辑卡的 `mtgnissa_card.name` 不区分大小写 A–Z 排序，同名时按 `oracle_id` 升序兜底，最后执行游标分页。不得先从全表任意选取代表印刷再应用原子条件，以免丢失仅由其他牌面或印刷记录满足的结果。服务端游标必须编码规范化后的英文卡名排序值与 `oracle_id` 位置，以完整延续该复合顺序；`oracle_id` 作为 `CardSummaryDTO` 的只读属性正常返回，游标对客户端仍是不透明字符串。
- 搜索命中计算直接使用 `mtgnissa_card -> 翻译、卡面展开表或 Oracle 数据`，不从语言印刷表计算命中。每个命中的 `oracle_id` 在其全部基础印刷中按 `mtgnissa_card.scryfall_id ASC` 选定代表印刷，再取其主牌面（单面 `face_index = -1`，多牌面 `face_index = 0`）作为摘要、名称和排序基准，不按发行时间选最新版本。仅生成详情入口时，再解析该代表主牌面实际存在的语言印刷行，按 `scryfall_language.sort_order ASC, language_code ASC, uuid ASC` 取首项，返回其 `printing_uuid` 和 `language`；不因此变更代表基础印刷或搜索命中。图片仍按该代表基础印刷解析，不能按客户端自行选择语言改写搜索结果图。不存在可定位的主牌面语言印刷属于 Card DB 关联错误，返回 `503 dependency_unavailable`，不得以基础卡面 UUID 冒充 `printing_uuid`。整个批次使用集合查询或批量关联，不逐项追加查询；具体版本、系列、语言和发行日期不参与全局结果顺序，客户端不重新选择版本。
- 全局搜索只以具有非空 `oracle_id` 的卡牌为逻辑结果；`oracle_id = null` 的记录不进入全局名称或语法搜索结果。具体 Set 卡牌列表仍可按 `printing_uuid` 展示这类记录。

## 7. 收藏与愿望单

### 7.1 `POST /api/v1/cards/{printing_uuid}/collection_adjustments`

本接口必须使用 `Idempotency-Key`。缓存有效期内的重复请求不再次应用 `delta`；服务端重新查询当前具体印刷及语言记录、整个收藏和所属系列的权威汇总后返回同一响应结构。响应不返回本次应用的增量；调用方必须始终以响应中的 `group`、`collection` 和 `set_summary` 更新显示。

请求：

```json
{
  "finish": "normal",
  "delta": 1
}
```

- `finish`、`delta` 必填；请求不接受合并方式或收藏列表筛选条件。
- `delta` 只能是 `1` 或 `-1`。
- 本接口始终只调整路径 `printing_uuid` 对应的具体印刷及语言记录，不得按 `oracle_id`、Set、收藏编号、语言筛选或收藏列表合并键扩大操作范围。增加和减少都作用于该记录的指定 `finish`；数量不足时返回 `409 collection_underflow`，不得改扣其他印刷或语言记录，也不得只应用部分增量。
- 卡牌详情页始终可以调用本接口。“我的卡牌”列表仅在 `merge_sets = false` 且 `merge_languages = false` 时显示加减控件；任一合并开关开启时卡位为只读合计，不得调用本接口。
- 后端必须锁定当前收藏记录并校验新数量不小于 0，在同一 App DB 事务中更新收藏并写一条操作日志；不得根据 Card DB 原始工艺属性拒绝对 `normal` 或 `foil` 计数桶的调整。
- 两种数量都归零时删除 `collection_item`；响应仍返回两个零值。

响应 `200`：

```json
{
  "printing_uuid": "00000000-0000-0000-0000-000000000000",
  "oracle_id": "00000000-0000-0000-0000-000000000010",
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
    "set_code": "KLD",
    "total_quantity": 8,
    "distinct_cards": 6
  }
}
```

`group` 是路径 `printing_uuid` 这一具体印刷及语言记录的权威普通／闪卡数量，名称为兼容当前响应结构而保留，不表示合并列表分组；`collection` 是整个收藏的权威页头合计；`set_summary` 是该具体记录所属系列的权威汇总，卡牌缺少系列时为 `null`。两个 `distinct_cards` 都按 `oracle_id` 去重，缺少 `oracle_id` 时按各自内部卡牌标识独立计种，不受列表合并开关影响。成功响应不要求列表重查；调用方以这些对象更新当前未合并卡位或详情收藏面板、页头和当前可见的系列标头，并在本次页面会话中保持原卡图、位置和排序不变。具体记录的两种数量归零时也保留至重新进入页面；重新进入后再按权威列表重新选择展示卡和排序。

### 7.2 `POST /api/v1/cards/{printing_uuid}/wishlist_state`

本接口必须使用 `Idempotency-Key`。缓存有效期内的重复请求不再次写愿望单或日志，服务端重新查询并返回当前 `included` 状态。

请求以目标状态表达加入或移出愿望单：

```json
{"included": true}
```

- `included` 必填。
- `included = true` 表示加入，`false` 表示移出。
- 状态实际改变时，在同一事务中更新愿望单并写对应操作日志。
- 当前状态已经等于目标状态时仍返回成功，但不重复写日志。

响应 `200`：`{"printing_uuid":"00000000-0000-0000-0000-000000000000","oracle_id":"00000000-0000-0000-0000-000000000010","included":true}`。没有逻辑身份时 `oracle_id` 为 `null`。

### 7.3 `GET /api/v1/collection/options`

返回当前收藏数量大于零的动态筛选选项：

```json
{
  "sets": [{"code": "KHM", "display_name": "凯德海姆"}],
  "languages": [{"code": "en", "display_name": "英文", "sort_order": 1}],
  "colors": ["W", "U", "B", "R", "G", "multicolor", "colorless"],
  "mana_values": ["0", "1", "2", "3", "4", "5", "6", "7", "8_plus"],
  "rarities": ["common", "uncommon", "rare", "mythic", "special", "bonus"]
}
```

`sets` 按系列发行时间从新到旧排列，发行时间相同或缺失时按 `set_code` 升序兜底；无发行时间的系列排在有发行时间的系列之后。`languages` 按 `sort_order` 升序、`code` 升序兜底。`colors`、`mana_values` 和 `rarities` 使用响应示例所列固定顺序。

### 7.4 `GET /api/v1/collection`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 按关联的 `mtgnissa_card.name` 模糊查询 |
| `set_code`、`language`、`color`、`mana_value`、`rarity` | string[]? | 各参数均可省略，省略表示不限制对应维度；同字段 OR，不同字段 AND |
| `sort` | enum? | 可省略，默认 `set`；可选值为 `set`、`color`、`mana_value`、`name` |
| `merge_sets` | boolean? | 可省略，默认 `false`；`sort=set` 时必须为 `false` |
| `merge_languages` | boolean? | 可省略，默认 `true` |
| `cursor` | string? | 下一批游标 |

`mana_value=0` 至 `mana_value=7` 分别匹配 `mtgnissa_card.cmc` 等于对应数值，`mana_value=8_plus` 匹配 `cmc >= 8`。稀有度请求值保持 `common`、`uncommon`、`rare`、`mythic`、`special`、`bonus`；前端依次显示为“铁、银、金、秘稀、特殊、额外”。

收藏分组键固定如下。表中的括号表示带首项类型标记的概念元组，不是把字段直接拼接成一个字符串：

| `merge_sets` | `merge_languages` | `oracle_id` 非空时的分组键 | 分组语义 |
| --- | --- | --- | --- |
| `false` | `false` | `("print", printing_uuid)` | 每个具体 Set、收藏编号和语言记录独立展示 |
| `true` | `false` | `("oracle_language", oracle_id, language.code)` | 同一逻辑卡、同一语言跨 Set 和收藏编号合并 |
| `false` | `true` | `("oracle_printing", oracle_id, set_code, collector_number)` | 同一逻辑卡、同一 Set 和收藏编号跨语言合并；同 Set 异画或不同收藏编号不得合并 |
| `true` | `true` | `("oracle", oracle_id)` | 同一逻辑卡的全部 Set、收藏编号和语言合并 |

任一记录的 `oracle_id = null` 时，四种开关组合一律使用 `("print", printing_uuid)`，只能与自身分为一组；不得使用空 `oracle_id`、`set_code`、收藏编号或语言把多个无逻辑身份的记录合并。上述键只用于应用筛选后的收藏列表分组和 `CollectionItemDTO` 数量合计，不参与收藏调整；收藏调整始终精确作用于路径 `printing_uuid`。

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

展示卡图和详情目标由后端在当前分组候选中按发行日期从新到旧、同日按 `printing_uuid` 升序选择。前端使用返回的 `card.printing_uuid` 进入具体版本详情，并可用只读 `card.oracle_id` 判断逻辑卡身份。`group_set` 和 `group_summary` 在 `sort=set` 时共同提供系列标头信息。列表筛选先作用于收藏明细，再严格按上表对应的分组键求和；合并不会重复或丢失数量。系列标头汇总和页头汇总不随筛选缩小，分别表达该系列和整个收藏的真实持有量。

排序规则固定为：`set` 按系列发行日期从新到旧、系列内收藏编号自然升序；`color` 按白、蓝、黑、红、绿、多色、无色，颜色相同按英文卡名不区分大小写 A–Z；`mana_value` 按 `cmc` 从低到高，数值相同按英文卡名不区分大小写 A–Z；`name` 按英文卡名不区分大小写 A–Z。所有规则最终均以返回展示卡的 `card.printing_uuid` 升序兜底。

只有 `merged_sets = false` 且 `merged_languages = false` 的卡位渲染普通／闪卡数量加减控件，并以该项 `card.printing_uuid` 调用 7.1。任一字段为 `true` 时，卡位仍显示合并后的 `normal_quantity` 和 `foil_quantity`，但数量条只读，必须完全隐藏减号、加号及其热区；不得仅以禁用外观保留控件，也不得从合并代表卡的 `printing_uuid` 发起调整。由于 `merged_*` 表示当前有效视图开关，即使某个合并分组当前恰好只有一条明细，也仍按合并卡位处理。

### 7.5 `GET /api/v1/wishlist/options`

结构与收藏选项相同。系列和语言只从当前愿望项生成；`sets` 仍按系列发行时间从新到旧、`set_code` 升序兜底，`languages` 仍按 `sort_order` 升序、`code` 升序兜底。颜色、法术力和稀有度使用与收藏相同的固定选项、值域和顺序。

### 7.6 `GET /api/v1/wishlist`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 按关联的 `mtgnissa_card.name` 模糊查询 |
| `set_code`、`language`、`color`、`mana_value`、`rarity` | string[]? | 各参数均可省略，省略表示不限制对应维度；使用同名重复参数传递；同字段 OR，不同字段 AND |
| `sort` | enum? | 可省略，默认 `set`；可选值为 `set`、`color`、`mana_value`、`name` |
| `cursor` | string? | 下一批游标 |

筛选值域和四种排序规则与收藏列表相同；愿望单不接受合并参数。`sort=set` 时按系列分组，`group_set` 返回对应 `SetRefDTO`；其他排序使用连续平铺，`group_set` 为 `null`。每个 `WishlistItemDTO` 还包含 `card: CardSummaryDTO` 和恒为 `true` 的 `included: boolean`，二者不可为 `null`。页面响应使用通用游标结构。

## 8. 卡组

### 8.1 `GET /api/v1/decks`

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `q` | string? | 卡组名称不区分大小写模糊查询 |
| `sort` | enum? | 可省略，默认 `updated_desc`；可选值为 `updated_desc`、`created_desc`、`name_asc`、`card_count_desc` |
| `cursor` | string? | 下一批游标 |

所有卡组排序在主排序值相同时统一按 `deck_id` 升序兜底；游标必须包含该最终唯一键对应的位置。

列表项：

```json
{
  "deck_id": "12",
  "name": "Superfriends",
  "created_at": "2026-10-01T04:00:00Z",
  "updated_at": "2026-10-09T04:00:00Z",
  "cover_card": null,
  "section_counts": {"commander": 1, "mainboard": 99, "sideboard": 0, "maybeboard": 4},
  "counted_total": 100,
  "color_distribution": {"W": 12, "U": 7, "B": 0, "R": 3, "G": 9, "C": 2},
  "exception": {"type": "shortage", "distinct_cards": 2, "missing_quantity": 3}
}
```

`section_counts` 必须包含 `commander`、`mainboard`、`sideboard`、`maybeboard` 四个固定键；每个值是数据库已保存状态下该分区全部 `deck_card.quantity` 之和，不是不同逻辑卡数量或明细行数，空分区为 0。`counted_total` 是非负整数，并固定满足：

```text
counted_total
= section_counts.commander
  + section_counts.mainboard
  + section_counts.sideboard
```

`maybeboard` 不计入 `counted_total`。`cover_card` 类型为 `CardSummaryDTO | null`：后端读取已保存的 `deck.cover_oracle_id`，再按统一展示版本规则选取具体印刷并返回；未配置封面时为 `null`，不得从指挥官或主牌自动补选。前端在 `null` 时显示统一无卡图占位。

`color_distribution` 固定返回 `W`、`U`、`B`、`R`、`G`、`C` 六个键及非负整数值，值是各颜色分类的原始计权张数，不是百分比。计算范围只包含已保存的 `commander` 与 `mainboard`，不包含 `sideboard` 或 `maybeboard`；每条卡牌按 `deck_card.quantity` 计权。`W/U/B/R/G` 从具体展示印刷主牌面的 `colors_mask` 读取，多色牌的完整数量分别计入其包含的每一种颜色；`colors_mask = 0` 的牌不计入任何颜色，也不得据此计入 `C`。只有主牌面的 `mana_cost` 明确包含 `{C}` 或包含 `C` 的复合符号时，该牌的完整数量才计入 `C`；同一张牌出现多个此类符号也只按牌张数计一次。没有数量的键仍返回 `0`。因此六个值之和可能大于统计范围内的实际卡牌总张数，也不要求等于 `counted_total`。前端以六个值之和为分母计算比例带宽度；六个值全为 `0` 时不得计算比例或绘制任何颜色段，比例带只保留未填充的容器背景。这里的 `C` 是明确的无色法术力分类，不表示“任意颜色”，也不表示通用法术力。

`exception` 类型为 `object | null`：没有异常时为 `null`；存在异常时 `type` 为 `shortage` 或 `deck_usage`，数量不足优先，`distinct_cards` 和 `missing_quantity` 均为非负整数。

卡组列表项中的 `section_counts`、`counted_total`、`color_distribution`、`cover_card` 和 `exception` 全部基于请求发生时 App DB 中的卡组已保存状态生成，不读取或合并任何浏览器中的未保存编辑态。编辑页的本地加减数量、拖拽、增加卡牌和代码导入结果，只有保存成功并在后续列表请求中读取到新状态后，才影响这些字段。

### 8.2 `POST /api/v1/decks`

本接口必须使用 `Idempotency-Key`。缓存有效期内的重复请求不得再次创建卡组；服务端使用内存记录中的新 `deck_id` 查询并返回该卡组当前的名称，同时重新生成指向该卡组的 `Location`。首次创建返回 `201`；去重后的重复请求返回 `200`。如果新卡组后来已被删除，重复请求返回 `404 deck_not_found`。

创建空白卡组并写“创建卡组”日志。请求：

```json
{"name": "新建卡组"}
```

`name` 必填，并按公共卡组名称边界校验。卡组列表的“新建卡组”操作固定以 `{"name":"新建卡组"}` 调用本接口；这是新建空白卡组的初始名称。创建成功后前端进入编辑页并允许用户修改名称，修改后的名称通过保存接口提交。

响应 `201`，设置 `Location: /api/v1/decks/{deck_id}`：

```json
{"deck_id":"12","name":"新建卡组"}
```

### 8.3 `GET /api/v1/decks/{deck_id}`

详情和编辑页共用同一响应：

```json
{
  "deck_id": "12",
  "name": "Superfriends",
  "source_text": "About\nName Superfriends\n...",
  "cover_printing_uuid": "00000000-0000-0000-0000-000000000000",
  "created_at": "2026-10-01T04:00:00Z",
  "updated_at": "2026-10-09T04:00:00Z",
  "sections": [
    {
      "type": "commander",
      "quantity": 0,
      "cards": []
    },
    {
      "type": "mainboard",
      "quantity": 1,
      "cards": [
        {
          "printing_uuid": "00000000-0000-0000-0000-000000000000",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "quantity": 1,
          "type_group": "planeswalker",
          "card": {
            "printing_uuid": "00000000-0000-0000-0000-000000000000",
            "oracle_id": "00000000-0000-0000-0000-000000000010",
            "name": "Nissa, Vital Force",
            "display_name": "生机妮莎",
            "set": {"code":"KLD","name":"Kaladesh","display_name":"卡拉德许","released_at":"2016-09-30"},
            "collector_number": "163",
            "language": {"code":"en","display_name":"英文","sort_order":1},
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
            "is_land": false,
            "can_be_commander": true
          },
          "exception": {"type": "shortage", "missing_quantity": 1}
        }
      ]
    },
    {
      "type": "sideboard",
      "quantity": 0,
      "cards": []
    },
    {
      "type": "maybeboard",
      "quantity": 0,
      "cards": []
    }
  ],
  "summary": {
    "counted_total": 1,
    "collection_rate": 0,
    "shortage": {"distinct_cards": 1, "missing_quantity": 1},
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
| `DeckCardDTO` | `printing_uuid` | string | 否 | 与内嵌 `card.printing_uuid` 相同；请求回传该字段 |
|  | `oracle_id` | string | 否 | 与内嵌 `card.oracle_id` 相同；只读，不得放入保存请求 |
|  | `quantity` | integer | 否 | 当前分区编辑数量，最小为 1 |
|  | `type_group` | `deck_card_type_group` | 否 | 后端确定的单一牌列归类；只用于决定非指挥官牌显示在哪个固定类型列 |
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
|  | `colors`、`produced_mana`、`oracle_tags` | string[] | 否 | 无值时为 `[]` |
|  | `type_groups` | `deck_card_type_group[]` | 否 | 用于统计的多值类型集合；至少一项，去重后按类型归类优先级返回 |
|  | `is_land` | boolean | 否 | 是否为地牌 |
|  | `can_be_commander` | boolean | 否 | 是否符合本期“传奇生物、传奇鹏洛客或传奇载具可以进入指挥官区”的业务规则；不是“是否为传奇”的属性值 |

顶层 `cover_printing_uuid` 类型为 `string | null`。后端读取数据库中的 `deck.cover_oracle_id`，按与卡组牌张相同的统一展示版本规则选择具体印刷，并把选中记录的 `printing_uuid` 返回；未配置封面时为 `null`。非空值必须与 `sections` 中该封面逻辑卡对应的 `DeckCardDTO.printing_uuid` 一致，前端直接用它标记当前封面并初始化待提交值，不再接收或处理封面的 `oracle_id`。用户重新选择封面时，将待提交值改为被点击卡牌的 `printing_uuid`；保存成功返回的 `cover_printing_uuid` 可能因后端重新选择展示版本而与请求值不同，前端以响应为准。顶层 `sections` 必须按 `commander`、`mainboard`、`sideboard`、`maybeboard` 顺序返回四项。`summary.counted_total` 统计指挥官、主牌和备牌，备选不计入；`collection_rate` 范围为 0 至 1，`counted_total = 0` 时固定为 0。两个异常汇总对象均包含非负的 `distinct_cards` 和 `missing_quantity`，没有对应异常时仍返回两个零值。统计范围由前端根据外层 `DeckSectionDTO.type` 选择，不能通过删除非统计分区的原始属性来表达；同一逻辑卡同时位于不同分区时，各分区中的 `DeckCardDTO` 独立携带本区数量和同一份统计属性。

界面中的主牌可见区域同时承载 `commander` 和 `mainboard` 两个真实分区：`commander` 只显示在固定指挥官列，`mainboard` 显示在其余类型列；备牌、备选分别对应 `sideboard`、`maybeboard`。视觉上的同区展示不得合并或改写 API 分区，保存时仍提交四个真实分区。

`type_group` 与 `statistics.type_groups` 使用同一套后端权威派生结果，但职责不同。后端先合并同一逻辑卡全部牌面的英文 Oracle 类型：任一牌面命中 `Land`、`Creature`、`Planeswalker`、`Instant`、`Sorcery`、`Enchantment`、`Artifact` 时，分别把对应枚举加入 `statistics.type_groups`；多个牌面或同一牌面命中的结果去重。若全部牌面都没有命中上述七类，则返回唯一的 `other`。因此双面牌按复合类型处理，例如一面为 `Sorcery`、另一面为 `Land` 时，`statistics.type_groups = ["land", "sorcery"]`。

`DeckCardStatisticsDTO` 的 `cmc`、`mana_cost`、`colors` 和 `produced_mana` 固定取后端为该逻辑卡选定的具体展示印刷之主牌面记录，主牌面的判定与 `CardSummaryDTO` 相同：单面牌取 `face_index = -1`，多牌面牌取 `face_index = 0`。`is_land` 是例外：只要该逻辑卡任一牌面的英文 Oracle 主类型包含 `Land` 就返回 `true`，不能只检查主牌面。卡组详情、编辑上下文和代码解析补齐上下文必须使用同一规则。

`type_group` 从上述统计集合中按 `land → creature → planeswalker → instant → sorcery → enchantment → artifact → other` 的固定优先级取第一项。该优先级同时适用于单牌面多类型牌和多牌面牌，例如神器生物归入 `creature`，神器地归入 `land`。`commander` 分区中的牌在界面上始终进入指挥官列，不使用 `type_group` 决定当前位置；两个类型字段仍必须正常返回，用于统计以及牌张离开指挥官分区后的本地归列。前端不得解析 `type_line`、牌面或 `statistics.type_groups` 重新推导 `type_group`。

普通牌列按神器、生物、结界、瞬间、法术、鹏洛客、地的既定显示顺序排列，`other` 固定追加在地之后。页面中没有任何 `type_group = other` 的牌时不渲染该列；任一可见区域出现该类型时，三个区域共同保留最右侧 `other` 列坐标，仅在实际有牌的区域渲染标题和牌张。可用宽度足够时内容区向右扩展；不足时保持既定列宽和间距并启用区域横向滚动，不得换行、压缩卡图或缩小交互目标。编辑状态中加入或移除最后一张 `other` 牌时，前端同步显示或隐藏该列并重新计算溢出。

`can_be_commander` 由后端按卡面计算：同一个 `card_uuid` 对应的英文 Oracle 类型栏必须包含 `Legendary`，并且同时包含 `Creature`、`Planeswalker`、`Vehicle` 三个类型词元中的至少一个，才返回 `true`。不得把不同卡面的类型词元合并判断，也不得由前端解析英文或中文类型栏。该字段只表达本期允许进入 `commander` 分区的条件，不等同于完整的 Commander 赛制合法性校验。

- 卡组牌张使用具体展示版本的 `printing_uuid` 发起请求，同时把 `oracle_id` 作为只读属性返回。前端用 `oracle_id` 在本地判断同一逻辑卡、合并同分区数量，用 `printing_uuid` 展示、跳转和保存；后端保存前再次把 `printing_uuid` 解析为 `oracle_id`，不得信任客户端回传逻辑身份。
- 后端按“拥有版本 → 愿望单版本 → 最新印刷版本；同层发行日期降序；同日 `printing_uuid` 升序”选择用于展示和详情跳转的 `card`。
- 每个 `DeckCardDTO.statistics` 都返回前端计算统计图、概率表和分组所需的原始属性，包括备牌和备选区；前端当前只用指挥官和主牌的 `quantity` 计权。备牌或备选拖入主牌后直接使用随卡牌已有的属性重算，不得为拖拽再次请求后端或 Card DB。
- 详情统计中的整体法术力曲线及其类型、颜色、功能标签子曲线必须排除 `statistics.is_land = true` 的牌；因此任一牌面为地的多牌面牌整体排除，平均法术力值同样排除。颜色子曲线与颜色数量分布中的 `W/U/B/R/G` 读取主牌面的 `statistics.colors`，空数组不进入任何颜色；只有主牌面的 `statistics.mana_cost` 明确包含 `{C}` 或包含 `C` 的复合符号时才按牌张数进入无色，同一张牌含多个无色符号也只计一次。费用颜色同样由前端解析主牌面的 `statistics.mana_cost`：混血、非瑞克西亚等复合符号按其中包含的全部 `W/U/B/R/G/C` 分别各计 1 点；只有 `{C}` 或明确包含 `C` 的复合符号计入无色，通用数字、变量符号和“任意颜色”不得计入 `C`。产费颜色只按主牌面的 `statistics.produced_mana` 中明确返回的各颜色分别计权，只有数组明确包含 `C` 时才计入无色。
- 前端在首次加载响应、增加卡牌、数量变化、拖拽换区和代码导入成功后，统一对各区的每个类型列按“法术力值从低到高 → 颜色组合固定顺序 → 英文卡名不区分大小写 A–Z → `DeckCardDTO.printing_uuid` 升序”重新排序。后端不以响应数组顺序承担页面排序职责；前端编辑过程中不得依赖后端重新排序。
- `other_decks_quantity` 和 `other_decks_count` 是编辑页的稳定基线。前端按 `oracle_id` 汇总当前未保存编辑态中的指挥官、主牌和备牌数量，备选区不计入，然后计算 `all_decks_quantity = other_decks_quantity + current_edit_quantity`；`using_decks = other_decks_count + 1` 仅在 `current_edit_quantity > 0` 时成立，否则等于 `other_decks_count`。数量加减和分区拖拽只重算上述前端值，不重新请求后端。
- 响应中的 `all_decks_quantity`、`using_decks` 和 `exception` 只描述请求发生时当前卡组的已保存状态，供只读详情页直接展示；编辑页产生未保存修改后必须改用上述基线公式，不得继续把这三个已保存值当作当前编辑结果。
- `GET /api/v1/decks` 的列表项及其 `exception` 不使用上述前端编辑公式，也不增加 `other_decks_quantity` 或 `other_decks_count`。卡组列表始终由后端基于各卡组已保存内容计算最终异常，现有响应结构和异常优先级不变。

### 8.4 `POST /api/v1/decks/{deck_id}/save`

本接口不使用 `Idempotency-Key`。保存是用户主动确认的完整覆盖操作，不使用版本号或乐观锁阻断旧编辑态；旧页面提交的内容也视为用户可能有意恢复的卡组状态。

请求提交完整当前编辑态，不提交原始导入文本：

```json
{
  "name": "Superfriends upgraded",
  "cover_printing_uuid": "00000000-0000-0000-0000-000000000000",
  "sections": [
    {
      "type": "commander",
      "cards": [{"printing_uuid": "00000000-0000-0000-0000-000000000000", "quantity": 1}]
    },
    {
      "type": "mainboard",
      "cards": [{"printing_uuid": "00000000-0000-0000-0000-000000000000", "quantity": 2}]
    }
  ]
}
```

- 顶层 `name`、`cover_printing_uuid`、`sections` 必填；`cover_printing_uuid` 类型为 `string | null`，`null` 表示不设置封面。每个分区对象的 `type`、`cards` 必填，每个卡牌对象的 `printing_uuid`、`quantity` 必填。四种分区类型可以不全部出现，缺少的分区按空处理；`commander` 允许缺少或 `cards: []`，保存不得要求至少存在一张指挥官。同一分区内映射到相同 `oracle_id` 的多个 `printing_uuid` 不得形成重复明细，后端按逻辑卡合并数量并校验上限；数量必须为正整数。
- 后端必须把非空 `cover_printing_uuid` 解析为对应 `oracle_id`，并确认该逻辑卡同时存在于本次 `sections` 解析出的任一分区中，可以来自指挥官、主牌、备牌或备选区。封面逻辑卡不在本次卡组内容中时返回 `422 validation_failed`，`details[].field` 为 `cover_printing_uuid`、`reason` 为 `cover_card_not_in_deck`，不得自动改选其他牌或部分保存。前端删除封面逻辑卡在全部分区中的最后一条记录时必须把本地 `cover_printing_uuid` 同步清为 `null`。
- `sections[].cards[]` 和顶层 `cover_printing_uuid` 都只提交后端此前返回的具体卡牌 `printing_uuid`，由后端解析并验证对应 `oracle_id`。这些具体印刷标识不会作为 `deck_card` 或封面的持久化身份，也不承诺保存后继续展示该具体印刷。后端验证全部标识仍有效且具有 `oracle_id`，再按当前名称和真实分区内容生成标准 `source_text`，并把封面解析结果保存为 `deck.cover_oracle_id`；请求不接受客户端提供任何 `oracle_id`。成功响应中的展示 `printing_uuid` 仍由后端按统一展示版本规则重新选择，因此可以与保存请求提交的值不同。这是卡组不保存具体印刷版本的有意产品行为。
- 保存请求中的 `cover_printing_uuid` 不存在或已经失效时返回 `422 card_reference_invalid`，`details[].field` 为 `cover_printing_uuid`；记录存在但没有 `oracle_id` 时返回 `422 deck_card_not_supported` 并使用同一字段路径。任一失败都不允许部分保存。
- 保存请求中任一 `printing_uuid` 不存在或已经失效时返回 `422 card_reference_invalid`，以 `sections[n].cards[m].printing_uuid` 定位；记录存在但没有 `oracle_id` 时返回 `422 deck_card_not_supported` 并使用相同字段路径。任一失败都不允许部分保存。
- 保存前，后端必须按与 `can_be_commander` 相同的权威规则重新校验 `commander` 分区中的每张牌。任一牌不符合“传奇生物、传奇鹏洛客或传奇载具”条件时返回 `422 validation_failed`，以 `sections[n].cards[m].printing_uuid` 定位，并在该明细中返回 `reason: "commander_card_not_eligible"`；不得自动移入主牌，也不得部分保存。客户端回传内容不能绕过该校验。
- 不检查客户端读取卡组后的数据库状态，也不判断提交内容是否来自旧页面。同一 App DB 事务中锁定目标卡组、替换 `deck_card` 并更新名称、封面逻辑卡和标准代码；并发保存按取得目标卡组写锁的顺序串行执行，最后一次成功提交的完整编辑态成为当前权威状态。
- 规范化后的名称、封面逻辑卡、分区和数量均与当前权威状态相同时，不替换明细、不更新 `updated_at`，也不写“修改卡组”日志，直接返回当前完整详情。
- 成功返回更新后的完整卡组详情。

### 8.5 `POST /api/v1/decks/{deck_id}/copies`

本接口必须使用 `Idempotency-Key`。缓存有效期内的重复请求不得再次复制卡组；服务端使用内存记录中的新 `deck_id` 查询并返回该副本当前的名称，同时重新生成指向该副本的 `Location`。首次复制返回 `201`；去重后的重复请求返回 `200`。如果副本后来已被删除，重复请求返回 `404 deck_not_found`。

无请求体。复制卡组、已保存的 `cover_oracle_id` 及全部分区，名称以固定前缀 `复制 ` 开始，写一条“复制卡组”日志。名称最长 255 个 Unicode 码点；若 `复制 ` 加原名称超过上限，保留完整前缀并从原名称末尾截断至总长 255，不拆分 Unicode 码点。重复复制继续在当次原名称前增加前缀并应用相同截断规则，不要求名称唯一。

响应 `201`，`Location` 指向新卡组，并返回新卡组的 `deck_id` 和名称。

### 8.6 `POST /api/v1/decks/{deck_id}/delete`

本接口不使用 `Idempotency-Key`。

无请求体。服务端不校验版本；在同一 App DB 事务中锁定并删除卡组及级联明细，同时写“删除卡组”日志，日志保留删除前名称快照。

保存和删除并发操作同一 `deck_id` 时按取得目标卡组写锁的顺序串行执行：删除先成功提交时，随后的保存返回 `404 deck_not_found`；保存先成功提交时，随后的删除删除该次保存后的卡组。

响应 `200`：`{"deck_id":"12","deleted":true}`。

删除成功后再次请求同一 `deck_id` 时返回 `404 deck_not_found`，不缓存或重放首次删除结果。

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
          "printing_uuid": "00000000-0000-0000-0000-000000000000",
          "oracle_id": "00000000-0000-0000-0000-000000000010",
          "quantity": 1,
          "card": {
            "printing_uuid":"00000000-0000-0000-0000-000000000000",
            "oracle_id":"00000000-0000-0000-0000-000000000010",
            "name":"Sol Ring",
            "display_name":"阳光戒",
            "set":{"code":"CMM","name":"Commander Masters","display_name":"指挥官大师","released_at":"2023-08-04"},
            "collector_number":"396",
            "language":{"code":"en","display_name":"英文","sort_order":1},
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

失败不得返回部分解析结果。成功时固定返回四个分区且空分区为 `[]`；顺序与输入文本无关，固定为 `commander`、`mainboard`、`sideboard`、`maybeboard`。输入代码可以省略 `Commander` 段，解析结果中的 `commander.cards` 此时为 `[]`，不得因为没有指挥官而报错。卡牌行中的 Set 代码、收藏编号和 `*F*` 被有意忽略，不能指定具体印刷、语言或工艺；后端直接按 `mtgnissa_card` 的标准英文名称、数量和分区解析逻辑卡，再按统一规则选择展示版本并返回主牌面 `printing_uuid`。命中记录没有 `oracle_id` 时整次返回 `422 deck_parse_failed`，以原代码行定位并返回 `reason: "deck_card_not_supported"`，不把 UUID 当作 Oracle 身份保存。`Commander` 分区中的每张牌都按 `can_be_commander` 同一规则校验；不符合资格时整次解析失败，对应行的 `reason` 为 `commander_card_not_eligible`，不返回或应用部分解析结果。

响应 `name` 类型为 `string | null`。代码中存在合法、非空且未超出长度上限的 `About`／`Name ` 行时返回解析出的规范化名称；没有 `Name ` 行，或 `Name ` 后为空白、超出长度上限时返回 `null`。`null` 表示“未提供有效的名称覆盖”，前端必须保留读取前的当前名称输入值，不得把名称设为 `null`，也不得清空或回退为已保存名称。名称元数据无效不得影响卡牌解析，也不得导致整次解析失败。

解析响应只负责表达卡名解析结果、分区、数量和后端选定的展示印刷，不返回拥有量、其他卡组占用或完整统计属性。前端收到成功结果后，对响应中的 `printing_uuid` 去重并排除已经缓存的编辑上下文，再按每批最多 200 项调用 `POST /api/v1/decks/{deck_id}/card_contexts`。只有所有批次均成功且返回项完整时，前端才一次性替换当前未保存编辑态；解析请求或任一上下文批次失败时均不得部分覆盖原编辑内容。

### 8.8 `GET /api/v1/decks/{deck_id}/cards/search`

查询参数：`q` 可选、`cursor` 可选，不接受 `limit`。`q` 去除首尾空白后最长 512 个 Unicode 字符，无最小长度或字符类型限制；空白或省略表示不加名称条件，仍可查询全部可加入卡组的候选。直接以 `mtgnissa_card_printing.printed_name` 执行文字包含匹配 `%q%`，通过 `card_uuid` 关联基础卡面，只保留 `oracle_id` 非空的命中，再按 `oracle_id` 去重。`deck_id` 不存在时返回 `404 deck_not_found`。

正式搜索只在点击“搜索”按钮或点击名称联想项时主动触发；输入变化本身只可能请求 8.10 的 `q%` 联想，不自动请求本接口。按钮不受联想最小长度限制；重复提交同一查询期间锁定按钮。关键字变化后尚未提交时继续保留上次正式结果，新查询成功后替换结果、更新计数并重置结果视口。

响应使用通用游标结构，`total` 是名称条件下按 `oracle_id` 去重后的完整数量；每项为 `DeckCardCandidateDTO`，固定包含 `card: CardSummaryDTO`、`display_type_line: string` 和可空的 `display_oracle_text: string | null`。名称查询命中的语言版本只用于确定 Oracle 候选集合；展示版本仍由后端按“拥有版本 → 愿望单版本 → 最新印刷版本”选取，返回其主牌面语言印刷 `printing_uuid`。按候选主牌面英文名不区分大小写 A–Z、`oracle_id ASC` 排序并生成绑定关键字的游标；前端使用只读 `oracle_id` 判断主牌中是否已存在该卡。空 Oracle 卡不返回，也不得通过上下文、导入或保存绕过限制。

候选搜索保持轻量，不返回拥有量、其他卡组占用或完整统计属性。用户首次点击某个尚无编辑上下文的候选项“加入”时，前端先以该项 `card.printing_uuid` 调用下述批量上下文接口；成功后把牌加入本地编辑态，失败时不加入并打开统一错误弹窗。已经缓存上下文的逻辑卡再次加入时不重复请求。

### 8.9 `POST /api/v1/decks/{deck_id}/card_contexts`

这是只读批量查询，不修改卡组、不写操作日志，也不要求 `Idempotency-Key`。请求：

```json
{
  "printing_uuids": [
    "00000000-0000-0000-0000-000000000000"
  ]
}
```

`printing_uuids` 是必填字段，必须为去重后的非空数组，最多 200 项。响应保持请求顺序，每项为 `DeckEditCardContextDTO`：

```json
{
  "items": [
    {
      "requested_printing_uuid": "00000000-0000-0000-0000-000000000000",
      "printing_uuid": "00000000-0000-0000-0000-000000000000",
      "oracle_id": "00000000-0000-0000-0000-000000000010",
      "card": {
        "printing_uuid": "00000000-0000-0000-0000-000000000000",
        "oracle_id": "00000000-0000-0000-0000-000000000010",
        "name": "Sol Ring",
        "display_name": "阳光戒",
        "set": {"code": "CMM", "name": "Commander Masters", "display_name": "指挥官大师", "released_at": "2023-08-04"},
        "collector_number": "396",
        "language": {"code": "en", "display_name": "英文", "sort_order": 1},
        "released_at": "2023-08-04",
        "cmc": 1,
        "mana_cost": "{1}",
        "colors": [],
        "rarity": "uncommon",
        "layout": "normal",
        "images": {"front_url": "https://cards.scryfall.io/normal/front/...jpg", "back_url": null, "has_distinct_back": false}
      },
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
        "is_land": false,
        "can_be_commander": false
      }
    }
  ]
}
```

- `requested_printing_uuid` 原样回显请求数组当前位置的标识，只用于前端关联响应；`printing_uuid` 是后端根据解析出的 `oracle_id` 按统一规则重新选择的最终展示版本，两者允许不同。`card` 是完整 `CardSummaryDTO`，其 `printing_uuid`、`oracle_id` 必须与外层最终展示字段一致；卡图选择规则与卡组详情一致。
- 任一请求数组元素不存在或已经失效时，整个请求返回 `422 card_reference_invalid`，并以 `printing_uuids[n]` 定位原始元素，不返回部分结果。记录存在但 `oracle_id` 为空时返回 `422 deck_card_not_supported`，使用相同字段路径定位。
- `owned_quantity` 是当前逻辑卡实际拥有量；`other_decks_quantity` 和 `other_decks_count` 明确排除当前 `deck_id`，作为前端计算当前未保存编辑态下所有卡组合计及使用卡组数的稳定基线。
- `statistics` 直接复用 `DeckCardStatisticsDTO`；数量不属于该 DTO，前端始终使用卡牌在当前分区的 `quantity` 计权。卡组详情、代码解析后批量补齐上下文和首次增加卡牌三条路径取得的统计属性结构必须一致。
- 前端负责本地维护分区数量、同分区 `oracle_id` 合并、固定排序、总张数、异常、收集率和统计结果。卡组代码输入框只作为读取卡组配置的输入入口：读取成功后由解析结果生成前端卡牌列表，后续列表编辑不得反向更新输入框，前端也不维护标准代码预览。保存时后端以请求中的名称和四个真实分区为权威内容，重新生成标准 `source_text` 并入库。上下文只在代码解析成功、首次加入尚未缓存的逻辑卡，或缓存被明确失效时批量取得；加减数量、拖拽分区和重复加入已缓存卡牌不得逐次请求后端。
- Card DB 或 App DB 查询失败时返回统一错误信封；前端保留请求前的未保存编辑态。

### 8.10 `GET /api/v1/decks/{deck_id}/cards/name_suggestions`

增加卡牌输入框的只读名称联想，不修改卡组、不写日志，不要求 `Idempotency-Key`。仅接受可选 `q`，不接受 `cursor`、`limit`、语言、筛选或排序参数；`deck_id` 不存在时返回 `404 deck_not_found`。

`q` 去除首尾空白后最长 512 个 Unicode 字符；输入至少包含 3 个英文字母（`A–Z/a–z`），或至少包含 2 个中文汉字（Unicode Han）时才查询，两个门槛均包含等号。数字、空格及标点不计入门槛；中英文混合输入分别计数，满足任一门槛即可。未达到条件或为空时正常返回 `200` 和空 `items`，不报最小长度错误，也不执行名称联想数据库查询。

达到门槛时按 `mtgnissa_card_printing.printed_name` 执行不区分大小写的文字前缀匹配 `q%`，通过基础卡面排除无 `oracle_id` 的记录；按相同名称比较口径去重，名称升序、原值稳定兜底，最多返回 5 个非空名称。不返回印刷版本数组、Oracle 对象、主结果计数或游标，响应为 `CardNameSuggestionsDTO`：

```json
{
  "items": [
    {"printed_name": "妮莎，生命之力"},
    {"printed_name": "妮莎的誓约"}
  ]
}
```

`items: CardNameSuggestionDTO[]` 为必填非空值，元素唯一字段 `printed_name: string` 必填非空；无匹配返回 `[]`，长度最多 5。联想不替换或刷新正式搜索结果。前端短暂防抖后请求并忽略过期响应，未满足门槛时关闭下拉框；点击一项后回填完整 `printed_name`、关闭联想，并立即以该完整名称调用 8.8 的 `%q%` 正式搜索，与点击搜索按钮等效。联想下拉覆盖在输入框下方，不改变弹窗或结果区尺寸；原型把联想与悬停气泡绘在同一画板只为展示独立状态，不代表实际同时出现。`printed_name` 普通索引用于此联想；正式 `%q%` 搜索按已确认的主动低频查询实现，不要求另建搜索引擎。

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

排序固定为 `operation_date DESC`，同一日期内按 `action_type` 的八种操作类型枚举顺序。

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
        "printing_uuid": "00000000-0000-0000-0000-000000000000",
        "name": "Lightning Bolt",
        "set_code": "M11",
        "collector_number": "146",
        "language": "en",
        "language_display_name": "英文",
        "finish": "normal",
        "quantity_delta": 1,
        "images": {"front_url": "https://cards.scryfall.io/normal/front/...jpg", "back_url": null, "has_distinct_back": false}
      }
    }
  ],
  "page": 1,
  "page_size": 10,
  "total_items": 3,
  "total_pages": 1
}
```

愿望单日志的 `finish` 和 `quantity_delta` 为 `null`。卡牌日志使用 `OperationCardSnapshotDTO`，完整字段为示例中 `card` 的全部键：`printing_uuid`、`name`、`set_code`、`collector_number`、`language`、`language_display_name` 均为非空字符串，`images` 为非空的 `CardImageDTO`；普通收藏日志的 `finish` 为 `finish_kind`、`quantity_delta` 为 `1` 或 `-1`。`printing_uuid` 是操作时具体语言印刷版本的主牌面业务标识，用于识别操作版本，日志明细行保持只读，不因此新增卡牌详情跳转；图片来源单独解析并写快照，仅图片可回退，不能改写 UUID 或语言。`language_display_name` 直接记录操作时 `scryfall_language.display_name`，前端不维护语言映射。读取历史日志只使用快照，不重新查询 Card DB 来改变文字、语言、图片或版本身份；原记录失效时详情入口可以返回 `404 card_not_found`，历史日志仍可展示。卡组动作将 `kind` 设为 `deck`，以 `deck: {"deck_id":"12","name":"Superfriends"}` 代替 `card`；快照展示不依赖当前卡组仍存在。

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

`error` 非空时必须包含 `stage` 和面向运维但不泄露 SQL、路径或堆栈的 `message`；只有源文件错误才返回相对文件名 `file`，只有可定位到输入行时才返回从 1 开始的 `line`。`verifying` 校验临时表行数和关联完整性，不读取或校验 App DB 中的个人数据引用。

## 11. 状态码与错误代码

| HTTP | `error.code` | 场景 |
| ---: | --- | --- |
| 400 | `invalid_request` | JSON、参数类型、未知字段或参数组合错误 |
| 400 | `invalid_cursor` | 游标无效、过期或与当前查询条件不匹配 |
| 400 | `invalid_date_range` | 开始日期晚于结束日期 |
| 400 | `confirmation_required` | 卡牌加载确认文本错误 |
| 400 | `idempotency_key_required` | 收藏调整、愿望单状态、创建卡组或复制卡组缺少或未正确填写 `Idempotency-Key` |
| 403 | `load_disabled` | 卡牌数据加载未启用 |
| 404 | `not_found` | 路由不存在 |
| 404 | `set_not_found` | Set 不存在 |
| 404 | `card_not_found` | 路径参数中的 `printing_uuid` 不存在或已失效；不用于请求体字段校验 |
| 404 | `deck_not_found` | 卡组不存在 |
| 404 | `task_not_found` | 加载任务不存在 |
| 409 | `load_in_progress` | 已有加载任务运行 |
| 409 | `collection_underflow` | 收藏减少后将小于零 |
| 409 | `idempotency_conflict` | 缓存有效期内同一幂等键已用于不同方法、路径或请求体 |
| 405 | `method_not_allowed` | 路径存在但 HTTP 方法不受支持 |
| 413 | `request_too_large` | 请求体超过服务端限制 |
| 415 | `unsupported_media_type` | 带请求体的接口未使用 `application/json` |
| 422 | `validation_failed` | 字段值或业务规则不合法 |
| 422 | `card_reference_invalid` | 保存卡组或查询编辑上下文时，请求体中的 `printing_uuid` 不存在或已失效；`details[].field` 定位具体元素 |
| 422 | `deck_parse_failed` | 卡组代码存在无法识别或歧义行 |
| 422 | `deck_card_not_supported` | 请求的 `printing_uuid` 没有逻辑卡牌身份，不能加入卡组 |
| 500 | `internal_error` | 未预期服务端错误 |
| 503 | `dependency_unavailable` | Card DB 或 App DB 暂不可用 |

全局卡牌搜索无法完整识别语法时不是错误，必须回退名称查询并返回 `200`。合法查询零结果、空收藏、空愿望单、空卡组和空日志同样返回 `200`。

## 12. 事务与一致性

- 收藏数量变化及其日志必须在同一 App DB 事务中提交。
- 愿望单状态变化及其日志必须在同一事务中提交；重复的幂等请求不产生日志。
- 卡组创建、复制、修改、删除及其日志必须分别在单一事务中提交。
- 卡组保存必须先完成全部 Card DB 标识验证，再开始 App DB 写事务，不允许部分保存。
- Card DB 与 App DB 不执行跨库事务或跨库外键；Card DB 读取失败时不得继续产生依赖该读取的个人数据写入。
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
| 2 卡牌查看与检索 | `GET /sets/options`、`GET /sets`、`GET /cards`、`GET /sets/{set_code}/tokens`、`GET /search` |
| 3 卡牌详情 | `GET /cards/{printing_uuid}` |
| 4 我的卡牌 | `GET /collection/options`、`GET /collection`、`POST /cards/{printing_uuid}/collection_adjustments` |
| 5 愿望单 | `GET /wishlist/options`、`GET /wishlist`、`POST /cards/{printing_uuid}/wishlist_state` |
| 6 卡牌中心复合 | 复用卡牌列表、收藏、愿望单和详情接口 |
| 7 卡组列表 | `GET/POST /decks`、`POST /decks/{deck_id}/copies` |
| 8 卡组编辑 | `GET /decks/{deck_id}`、`POST /decks/{deck_id}/save`、`POST /decks/{deck_id}/delete`、`POST /decks/{deck_id}/parse_code`、`GET /decks/{deck_id}/cards/search`、`GET /decks/{deck_id}/cards/name_suggestions`、`POST /decks/{deck_id}/card_contexts` |
| 9–13 卡组展示版本、拥有量、异常、使用情况与统计 | `GET /decks/{deck_id}`、`GET /cards/{printing_uuid}` |
| 14 操作日志 | `GET /operation_logs/summary`、`GET /operation_logs` |
| 15 加载卡牌数据 | `POST /card_data/load`、`GET /card_data/load/{task_id}` |

表中省略的路径均以 `/api/v1` 为前缀。
