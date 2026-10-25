# MTGNissa 个人数据库设计

## 1. 设计范围

本文定义 App DB（个人数据库）的业务数据结构，覆盖收藏、愿望单、卡组和用户操作日志。卡牌正文、系列、翻译、规则和标签仍只存在于只读 Card DB，不复制到 App DB。

系统采用单用户、单实例模型，因此不创建 `users` 或 `app_profile` 表，也不在业务表中增加 `user_id`。页面用于区分不同部署的名称来自可选配置 `app.display_name`，默认是空字符串并按原值透传，不属于业务数据。

所有表使用 InnoDB 和 `utf8mb4_unicode_ci`，结构不依赖数据库执行 `CHECK` 约束，以兼容 MySQL 5.7、MySQL 8 和相应 MariaDB 版本。下列表格中只有明确标记 `NULL` 的列允许为空，其余列均为 `NOT NULL`。数据库中的业务时间统一使用 `DATETIME(6)` 并写入 UTC；需要按自然日展示或查询时，由应用根据部署时区换算，不额外持久化本地日期。

## 2. 标识选择和数据库边界

### 2.1 两种卡牌标识

| 业务对象 | 稳定标识 | 原因 |
| --- | --- | --- |
| 具体印刷及语言版本 | `printing_uuid CHAR(36)` | 取 `mtgnissa_card_printing.uuid`；单面使用自身语言行 UUID，多牌面统一使用同一印刷、同一语言主牌面（`face_index = 0`）的语言印刷 UUID，收藏和愿望单不按牌面拆分。 |
| 逻辑卡牌 | `oracle_id CHAR(36)` | 跨 Set、收藏编号和语言归并同一张牌，适合卡组记录和收藏汇总。 |

`mtgnissa_card.uuid` 是基础卡面标识，用于 `card_uuid` 关联及翻译映射；普通单面 `face_index = -1`，多牌面按 `0..4` 组织。建立个人引用时必须定位有效的主牌面语言印刷行，不能直接用任意基础卡面 UUID 或背面语言行创建独立收藏。基础语言的 UUID 可能与基础卡面 UUID 相同，是否有效仍以语言印刷行及主牌面校验为准。

少数没有 `oracle_id` 的普通卡牌仍可加入收藏或愿望单，但不能加入按逻辑卡牌记录的卡组。收藏汇总时这类记录使用 `printing_uuid` 作为独立分组键，不能把所有 `NULL oracle_id` 合并到一起。

图片 ID `scryfall_id` 不承担个人数据身份。语言印刷行缺少图片 ID 时，仅图片来源可回退关联的 `mtgnissa_card.scryfall_id`，不改写版本 UUID、语言、分组或个人状态。系列以 `mtgnissa_set.code` 定位，不保存 Set UUID。Token 不进入本设计的收藏、愿望单、卡组或卡牌操作日志。

### 2.2 不建立跨数据库外键

Card DB 和 App DB 使用独立连接，可能部署在不同主机。App DB 只保存卡牌 ID 和必要的引用快照，不对 Card DB 建物理外键。写入收藏、愿望单或卡组前，由应用先从 Card DB 验证标识并取得快照，然后在 App DB 事务中写入。

`card_print_ref` 不是第二份卡牌元数据库。它只保存个人数据实际引用过的印刷版本及分组所需字段；展示用名称、费用、类型、颜色、发行日期和图片由 Card DB 批量补齐。操作日志例外：写入时保存必要的名称、语言与图片快照，之后读取历史不依赖 Card DB。普通版本信息通过 `mtgnissa_card_printing -> mtgnissa_card -> 翻译或 Oracle 数据` 取得，主牌面 UUID 与语言在写入前验证。

## 3. 关系概览

```text
card_print_ref
  ├── 0..1 collection_item
  └── 0..1 wishlist_item

deck
  └── 0..N deck_card             (按 section_type + oracle_id)

operation_log
  ├── 0..1 operation_card_detail
  └── 0..1 operation_deck_detail
```

共 8 张表。不创建用户资料表、缓存统计表、卡组具体印刷版本表或每日日志汇总表；封面只在 `deck` 保存逻辑卡 ID，具体印刷仍实时选择，其他项目也都是配置项或可从当前数据计算出的派生结果。

## 4. 表结构

### 4.1 `card_print_ref`：个人数据引用的印刷版本

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `printing_uuid` | `CHAR(36)` | 主键；具体印刷及语言版本。 |
| `oracle_id` | `CHAR(36) NULL` | 逻辑卡牌 ID；普通卡缺少 Oracle 身份时允许为空。 |
| `set_code` | `VARCHAR(16)` | 所属系列代码，对应 `mtgnissa_set.code`，用于引用和分组。 |
| `collector_number` | `VARCHAR(64)` | 收藏编号快照；不能转为数字。 |
| `lang` | `VARCHAR(8)` | 原语言代码，取语言印刷行 `language_code`；UI 统一显示为“语言”。 |
| `created_at` | `DATETIME(6)` | 首次被个人数据引用的时间。 |
| `last_verified_at` | `DATETIME(6)` | 最近一次从 Card DB 验证成功的时间。 |

普通索引：`(oracle_id)`、`(set_code, lang, collector_number)`、`(lang, oracle_id)`。系列复合索引以 `set_code`、`lang` 为前两列，支持只指定系列或同时指定系列与语言的查询；收藏编号保留在末尾用于版本分组，不承担自然排序。

每个 `printing_uuid` 唯一指向一条主牌面语言印刷记录；后端按其基础卡面关系组织同一印刷、同一语言的完整牌面，并确认 Oracle、系列、收藏编号及语言一致。其他牌面不各自建立个人引用，也不因共用图片或图片缺失而合并不同语言版本。

### 4.2 `collection_item`：收藏数量

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `printing_uuid` | `CHAR(36)` | 主键；外键到 `card_print_ref`。 |
| `normal_quantity` | `INT UNSIGNED` | 普通卡数量，默认 0。 |
| `foil_quantity` | `INT UNSIGNED` | 闪卡数量，默认 0；蚀刻闪也归入此列。 |
| `created_at` | `DATETIME(6)` | 首次加入收藏时间。 |
| `updated_at` | `DATETIME(6)` | 最近数量变化时间。 |

应用保证 `normal_quantity + foil_quantity > 0`。两种数量都减到 0 时删除该行，所以“我的卡牌”只需查询本表已有记录。`UNSIGNED` 防止写入负数；应用还必须检查调整后的单列数量不小于 0。普通与闪卡是同一具体卡牌记录上的两个独立收藏计数桶，每个 `printing_uuid` 均允许维护两种数量，不根据 Card DB 的 `finishes_mask` 或其他原始工艺属性限制调整。

使用两个数量列而不是按工艺拆两行，正好对应界面的 `普通:闪*`，也能保证一个印刷版本的数量更新在单行锁内完成。

### 4.3 `wishlist_item`：愿望单

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `printing_uuid` | `CHAR(36)` | 主键；外键到 `card_print_ref`。 |
| `created_at` | `DATETIME(6)` | 加入愿望单时间。 |

一行只表达“该具体印刷及语言版本在愿望单中”。不保存数量、闪卡偏好、备注或优先级。收藏与愿望单相互独立，增加收藏不会删除愿望项。

### 4.4 `deck`：卡组

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `id` | `BIGINT UNSIGNED AUTO_INCREMENT` | 主键。 |
| `name` | `VARCHAR(255)` | 卡组名称；不要求唯一。 |
| `source_text` | `LONGTEXT` | 后端根据当前已保存的卡组名称和四个真实分区重新生成的标准卡组代码；不保存代码输入框中的原始导入文本。 |
| `cover_oracle_id` | `CHAR(36) NULL` | 用户选作卡组封面的逻辑卡 ID；只保存逻辑卡，不保存具体印刷版本。未配置封面时为 `NULL`。 |
| `created_at` | `DATETIME(6)` | 创建时间。 |
| `updated_at` | `DATETIME(6)` | 名称或内容最近修改时间。 |

索引：`(updated_at, id)`。卡组列表支持对 `deck.name` 执行去除首尾空白后、不区分大小写的模糊包含查询。不为名称增加索引：卡组数据量较小，而且前置通配符包含匹配不能直接利用普通 B-tree 名称索引。

保存流程接收前端提交的卡组名称、`cover_printing_uuid` 和四个真实分区；封面及分区卡牌都通过 `printing_uuid` 解析并校验对应 `oracle_id`。只要存在无效标识、缺少逻辑身份、数量非法、指挥官资格不符或封面逻辑卡不属于本次卡组内容，就返回错误，且不修改 `deck`、`deck_card` 或操作日志。后端校验成功后根据名称和真实分区生成标准 `source_text`，把封面解析结果保存到 `cover_oracle_id`，再在同一事务中更新卡组、替换明细并写一条“修改卡组”日志。

### 4.5 `deck_card`：卡组逻辑卡牌明细

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `deck_id` | `BIGINT UNSIGNED` | 外键到 `deck(id)`，删除卡组时级联删除。 |
| `section_type` | `VARCHAR(32)` | 固定分区代码：`mainboard`、`sideboard`、`commander` 或 `maybeboard`。 |
| `oracle_id` | `CHAR(36)` | 逻辑卡牌 ID。 |
| `quantity` | `INT UNSIGNED` | 编辑数量，必须大于 0。 |
| `name_snapshot` | `VARCHAR(512)` | 保存时的标准组合牌名，用于 Card DB 暂时缺失时诊断和展示。 |

主键：`(deck_id, section_type, oracle_id)`。反向统计索引：`(oracle_id, deck_id, quantity)`；分区展示索引由主键覆盖。`section_type` 的四个合法值由应用层枚举校验，数据库不使用 `CHECK` 或 `ENUM`，便于兼容和后续迁移。

卡组不保存 Set、语言、普通/闪、展示版本或实体卡分配。同一逻辑卡牌可以出现在多个分区，但在同一分区中只能有一行。分区不是用户可自定义实体，界面显示名称可以本地化，数据库中的固定分区代码及顺序为 `commander`、`mainboard`、`sideboard`、`maybeboard`。卡组代码只接受 `About`、`Commander`、`Deck`、`Sideboard`、`Maybeboard` 五个固定英文标题，不接受中文或其他本地化别名；解析后分别映射到元信息以及上述四个分区代码。没有任何分区标题的纯卡牌列表按 `Deck`／`mainboard` 读取；出现分区标题后，卡牌行必须归属于已识别的卡牌分区，未知标题必须报错。

### 4.6 `operation_log`：操作日志主表

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `id` | `BIGINT UNSIGNED AUTO_INCREMENT` | 主键。 |
| `action_type` | `TINYINT UNSIGNED` | 只允许下表中的 8 个代码。 |
| `occurred_at` | `DATETIME(6)` | 操作发生的 UTC 时间。 |

动作代码固定为：

| 代码 | 动作 |
| ---: | --- |
| 1 | 增加卡牌 |
| 2 | 减少卡牌 |
| 3 | 加入愿望单 |
| 4 | 移出愿望单 |
| 5 | 创建卡组 |
| 6 | 删除卡组 |
| 7 | 复制卡组 |
| 8 | 修改卡组 |

索引：`(occurred_at, action_type, id)`、`(action_type, occurred_at, id)`。查询仍可使用 `ORDER BY occurred_at DESC, id DESC`；索引定义不写 `DESC`，避免依赖数据库对降序索引的版本支持。

应用按照 `/app` 返回的部署时区，把 UTC `occurred_at` 转换为接口中的 `operation_date`，再按派生日期和 `action_type` 生成每日汇总；App DB 不保存 `operation_date`，也不单独维护汇总表。只有业务状态确实变化时才写日志；重复加入愿望单、数量增量为 0 或未改变内容的卡组保存不产生日志。

### 4.7 `operation_card_detail`：卡牌操作快照

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `operation_id` | `BIGINT UNSIGNED` | 主键；外键到 `operation_log(id)`。 |
| `printing_uuid` | `CHAR(36)` | 操作时的印刷版本 ID。 |
| `oracle_id` | `CHAR(36) NULL` | 操作时的逻辑卡牌 ID。 |
| `card_name` | `VARCHAR(512)` | 操作时按翻译后语言回退规则得到的组合牌名快照。 |
| `set_code` | `VARCHAR(16)` | 系列代码快照。 |
| `collector_number` | `VARCHAR(64)` | 收藏编号快照。 |
| `lang` | `VARCHAR(8)` | 原语言代码快照；UI 统一显示为“语言”。 |
| `lang_display_name` | `VARCHAR(64)` | 操作时从 `scryfall_language.display_name` 取得的语言展示名快照。 |
| `front_url` | `TEXT NULL` | 操作时解析的正面卡图 URL；缺图时为 `NULL`。 |
| `back_url` | `TEXT NULL` | 操作时解析的反面卡图 URL；无独立反面或缺图时为 `NULL`。 |
| `has_distinct_back` | `BOOLEAN` | 快照是否具备独立正反面卡图；由应用写入，缺图或单张卡图时为 `false`。 |
| `finish_kind` | `TINYINT UNSIGNED NULL` | 收藏操作为 1=普通、2=闪；愿望单操作为 `NULL`。 |
| `quantity_delta` | `BIGINT NULL` | 收藏操作保存有符号变化量；使用 `BIGINT` 才能覆盖 `INT UNSIGNED` 数量列的完整变化范围。愿望单操作为 `NULL`。 |

日志必须保存展示快照，不能依赖以后仍能从 Card DB 查到同一记录。写入卡牌日志时同时保存主牌面 `printing_uuid`、名称、系列、收藏编号、语言代码、语言展示名及图片快照；仅图片来源可回退，不能改写操作版本及语言。读取时直接使用快照，原版本失效时历史仍可展示，不能从当前 Card DB 补齐或改写历史，也不为日志明细行新增编辑或详情跳转。

HTTP 层将 `lang`、`lang_display_name` 映射为 `language`、`language_display_name`，将三个图片列组织为 `images: CardImageDTO`；不直接暴露持久化模型。收藏步进请求的 `delta` 只能为 `1` 或 `-1`，每次成功改变一个版本的一种数量桶写一条日志，不将连续点击合并成批量调整；愿望单日志的工艺和数量变化为空。

### 4.8 `operation_deck_detail`：卡组操作快照

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `operation_id` | `BIGINT UNSIGNED` | 主键；外键到 `operation_log(id)`。 |
| `deck_id` | `BIGINT UNSIGNED` | 原卡组 ID，仅供追踪；不对 `deck` 建外键。 |
| `deck_name` | `VARCHAR(255)` | 操作完成后的卡组名快照。 |

删除卡组后日志仍需保留，因此 `deck_id` 不建立外键。复制卡组记录新卡组的 ID 和名称；重命名归入“修改卡组”，只保存新名称。

## 5. 关键查询和派生结果

### 5.1 收藏四种合并方式

先用 `collection_item JOIN card_print_ref` 得到数量，分组维度如下：

| 合并 Set | 合并语言 | Oracle 非空时的分组键 |
| --- | --- | --- |
| 否 | 否 | `("print", printing_uuid)` |
| 是 | 否 | `("oracle_language", oracle_id, lang)` |
| 否 | 是 | `("oracle_printing", oracle_id, set_code, collector_number)` |
| 是 | 是 | `("oracle", oracle_id)` |

这些键是带类型标记的元组，不是字符串拼接。Oracle 为空时，四种组合均固定使用 `("print", printing_uuid)`，只与自身成组，不因系列、编号或语言相同而合并。

先从 App DB 取得有效收藏引用及数量，再按引用范围批量补齐 Card DB 的筛选、排序和展示信息。筛选先作用于具体收藏明细，再按上表分组，普通和闪卡分别 `SUM`；最终按服务端权威顺序生成稳定游标，不能先分页再筛选或分组。每组展示版本按发行日期从新到旧、`printing_uuid ASC` 兜底选择，数量调整始终精确作用于具体版本，不能使用汇总组的代表 UUID 调整数量。系列和页头真实持有量汇总不随筛选缩小。两个数据库可能跨主机，不执行跨库 SQL JOIN。

### 5.2 卡组拥有量与异常

按 `oracle_id` 汇总收藏两列之和得到实际拥有量。计算当前卡组数量和所有卡组编辑总数时，只汇总 `section_type IN ('mainboard', 'sideboard', 'commander')` 的 `deck_card.quantity`；同一张牌出现在一副卡组的多个计数分区时，使用卡组数仍只计算一次 `COUNT(DISTINCT deck_id)`。备选区只展示，不产生数量不足或卡组占用。当前卡组数量不足优先于卡组占用，判断公式直接采用需求文档第 11 节，不落库存分配表。

类型、颜色、Mana Value、曲线以及土地／非地统计从 Card DB 按统一牌面规则取得。后端先按统一展示版本规则为每个 `oracle_id` 选择具体印刷；普通单面牌以 `face_index = -1` 为主牌面，多牌面牌以 `face_index = 0` 为主牌面，`cmc`、`mana_cost`、`colors` 和 `produced_mana` 均读取该主牌面。`is_land` 不使用单一主牌面规则：同一逻辑卡任一牌面的英文 Oracle 主类型包含 `Land` 即为真。用于统计的多值 `statistics.type_groups` 合并该逻辑卡全部牌面命中的地、生物、鹏洛客、瞬间、法术、结界和神器类型并去重，全部未命中时使用 `other`；单值 `type_group` 再按需求及 API 契约规定的优先级从该集合中选取。卡组列表的总数、异常数和统计均根据 App DB 已保存状态实时派生；单用户规模下无需维护容易失真的缓存列。

### 5.3 卡组封面与自动展示版本

`deck.cover_oracle_id` 保存用户在编辑页选择的封面逻辑卡。卡组列表只在该字段非空且对应逻辑卡仍属于该卡组时返回封面；卡组详情和编辑响应也由后端按统一展示版本规则把该逻辑身份转换为 `cover_printing_uuid`，不向前端暴露封面逻辑 ID。未配置封面时返回空值并由前端显示统一无卡图占位，不从指挥官或主牌自动选择替代封面。

候选层级固定为收藏版本、愿望单版本、其他最新印刷版本。每层内使用确定性顺序：

1. `released_at` 较新优先；
2. `released_at` 相同时直接按 `printing_uuid` 升序选择。

具体印刷版本的选择结果不写入 App DB，每次展示时根据 `cover_oracle_id` 计算，因此收藏和愿望单改变后会立即生效，也不会被误解为实体卡分配。

## 6. 写入事务和一致性

- 调整收藏：锁定 `collection_item`，校验新数量，写入或删除收藏行，并写操作日志；全部在同一 App DB 事务中完成。
- 加入/移出愿望单：状态变化和日志在同一事务中完成。
- 创建、复制、修改、删除卡组：`deck`、`deck_card` 和一条日志在同一事务中完成。卡组保存是用户主动确认的完整覆盖操作，不使用版本号或乐观锁阻断旧编辑态；旧页面提交的完整内容同样按本次用户保存意图处理。
- `card_print_ref` 可在同一事务中 `INSERT ... ON DUPLICATE KEY UPDATE last_verified_at=...`；不能用新快照静默改变已有 `printing_uuid` 的身份字段，发现不一致时应报数据错误。
- 操作日志是追加数据，不提供普通业务删除或修改接口。若以后增加日志保留策略，应整条删除主表及其明细，不能留下半条事件。

只有收藏调整、愿望单状态、创建卡组和复制卡组要求 `Idempotency-Key`；幂等记录按 API 契约保存在单进程有界内存中，完成记录保留 30 分钟，不在 App DB 新建幂等表。重复请求不得重复改变状态或写日志；保存卡组、只读查询和卡牌加载不使用该键。

## 7. 不纳入设计的结构

- `users`、`app_profile`、角色和权限表：单用户实例不设计认证和权限体系；页面名称读取 `app.display_name` 配置。
- 每张实体卡一行的实例表：需求明确只记录汇总数量。
- 库存分配表：卡组占用只是统计，不代表实体牌分配。
- 卡组具体印刷版本列：只保存封面逻辑卡 `cover_oracle_id`，具体展示版本由实时规则自动选择。
- 日志每日汇总表：可从有覆盖索引的明细实时聚合。
- 价格、交易、品相、合法性、愿望数量和愿望备注相关表：均不在本设计范围。

## 8. 迁移落地顺序

Goose 建表迁移按依赖顺序各自创建一张表：`card_print_ref`、`collection_item`、`wishlist_item`、`deck`、`deck_card`、`operation_log`、`operation_card_detail`、`operation_deck_detail`。各表字段、主键和索引按第 4 节定义创建，包含主牌面 `printing_uuid` 引用、`deck.cover_oracle_id` 和卡牌日志图片快照。

MySQL DDL 会隐式提交，因此不把多张表放进同一个版本；迁移失败后从尚未完成的版本重试，回滚按版本反序执行。结构变更通过新增迁移版本表达，不改写已执行版本；首次部署依次执行全部版本，已有部署只执行尚未应用的版本。

迁移 SQL 统一位于 `migrations/app`，由 `migrations/embed.go` 内嵌到可执行文件。服务连接 Card DB 和 App DB 后、启动 HTTP 服务前自动升级 App DB；迁移失败时关闭数据库连接并终止启动。Goose 使用 App DB 中的 `goose_db_version` 表记录已执行版本。Card DB 不由 Goose 管理。

迁移中只对 App DB 内部关系使用具名外键，不使用 `CHECK`。数量总和、分区代码、动作代码、工艺代码以及操作明细与动作类型的匹配关系均由应用层集中校验；数值列继续使用 `UNSIGNED`、`NOT NULL` 和事务作为数据库层保护。

兼容性基线还包括：不使用原生 `JSON`、生成列、表达式索引、降序索引和依赖版本的 `DEFAULT` 表达式；所有时间与初始值由应用显式写入。App DB 的复合主键和普通索引在 `utf8mb4` 下均低于旧版 InnoDB 常见的 767 字节索引键限制。
