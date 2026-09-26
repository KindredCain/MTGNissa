# MTGNissa 个人数据库设计

## 1. 设计范围

本文定义 App DB（个人数据库）的业务数据结构，覆盖收藏、愿望单、卡组和用户操作日志。卡牌正文、系列、翻译、规则和标签仍只存在于只读 Card DB，不复制到 App DB。

当前系统是单用户、单实例，因此不创建 `users` 或 `app_profile` 表，也不在业务表中增加 `user_id`。页面用于区分不同部署的名称来自可选配置 `app.display_name`，默认是空字符串并按原值透传，不属于业务数据。如果以后改为单实例多用户，再在根实体上增加真正的 `owner_id`。

所有表使用 InnoDB 和 `utf8mb4_unicode_ci`，结构不依赖数据库执行 `CHECK` 约束，以兼容 MySQL 5.7、MySQL 8 和相应 MariaDB 版本。下列表格中只有明确标记 `NULL` 的列允许为空，其余列均为 `NOT NULL`。业务时间使用 `DATETIME(6)` 并统一写入 UTC；按日展示日志时，应用按部署时区计算并写入 `operation_date`。

## 2. 标识选择和数据库边界

### 2.1 两种卡牌标识

| 业务对象 | 稳定标识 | 原因 |
| --- | --- | --- |
| 具体印刷及语言版本 | `scryfall_id CHAR(36)` | 同时确定 Set、收藏编号和实体牌语言；同一多面牌的各牌面共享此 ID，不会重复计算收藏。 |
| 逻辑卡牌 | `oracle_id CHAR(36)` | 跨 Set、收藏编号和语言归并同一张牌，适合卡组记录和收藏汇总。 |

`scryfall_card.uuid` 是扁平化后的牌面记录 ID，只用于 Card DB 内部关联中文印刷文字，不能用作个人收藏主键，否则双面牌会被当作两张实体牌。

少数没有 `oracle_id` 的 Scryfall 对象仍可加入收藏或愿望单，但不能加入按逻辑卡牌记录的卡组。收藏汇总时这类记录使用 `scryfall_id` 作为独立分组键，不能把所有 `NULL oracle_id` 合并到一起。

### 2.2 不建立跨数据库外键

Card DB 和 App DB 使用独立连接，可能部署在不同主机。App DB 只保存卡牌 ID 和必要的引用快照，不对 Card DB 建物理外键。写入收藏、愿望单或卡组前，由应用先从 Card DB 验证标识并取得快照，然后在 App DB 事务中写入。

`card_print_ref` 不是第二份卡牌元数据库。它只保存个人数据实际引用过的印刷版本及分组所需字段；名称、费用、类型、颜色、发行日期和图片等展示信息始终以 Card DB 为准。

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

共 8 张表。不创建用户资料表、缓存统计表、卡组展示版本表或每日日志汇总表；这些都是配置项或可从当前数据计算出的派生结果。

## 4. 表结构

### 4.1 `card_print_ref`：个人数据引用的印刷版本

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `scryfall_id` | `CHAR(36)` | 主键；具体印刷及语言版本。 |
| `oracle_id` | `CHAR(36) NULL` | 逻辑卡牌 ID；允许 Scryfall 非 Oracle 对象为空。 |
| `set_id` | `CHAR(36)` | 系列稳定 ID。 |
| `set_code` | `VARCHAR(32)` | 系列代码快照，便于故障展示和诊断。 |
| `collector_number` | `VARCHAR(64)` | 收藏编号快照；不能转为数字。 |
| `lang` | `VARCHAR(16)` | 实体牌语言代码。 |
| `created_at` | `DATETIME(6)` | 首次被个人数据引用的时间。 |
| `last_verified_at` | `DATETIME(6)` | 最近一次从 Card DB 验证成功的时间。 |

索引：`(oracle_id)`、`(set_id, collector_number, lang)`、`(lang, oracle_id)`。

同一个 `scryfall_id` 在 Card DB 中可能因多面牌对应多行；建立引用时应聚合这些行，并确认 `oracle_id`、`set_id`、收藏编号和语言一致。

### 4.2 `collection_item`：收藏数量

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `scryfall_id` | `CHAR(36)` | 主键；外键到 `card_print_ref`。 |
| `normal_quantity` | `INT UNSIGNED` | 普通卡数量，默认 0。 |
| `foil_quantity` | `INT UNSIGNED` | 闪卡数量，默认 0；蚀刻闪也归入此列。 |
| `created_at` | `DATETIME(6)` | 首次加入收藏时间。 |
| `updated_at` | `DATETIME(6)` | 最近数量变化时间。 |

应用保证 `normal_quantity + foil_quantity > 0`。两种数量都减到 0 时删除该行，所以“我的卡牌”只需查询本表已有记录。`UNSIGNED` 防止写入负数；应用还必须检查调整后的单列数量不小于 0，并检查 Card DB 的 `finishes_mask` 是否支持相应工艺。

使用两个数量列而不是按工艺拆两行，正好对应界面的 `普通:闪*`，也能保证一个印刷版本的数量更新在单行锁内完成。

### 4.3 `wishlist_item`：愿望单

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `scryfall_id` | `CHAR(36)` | 主键；外键到 `card_print_ref`。 |
| `created_at` | `DATETIME(6)` | 加入愿望单时间。 |

一行只表达“该具体印刷及语言版本在愿望单中”。不保存数量、闪卡偏好、备注或优先级。收藏与愿望单相互独立，增加收藏不会删除愿望项。

### 4.4 `deck`：卡组

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `id` | `BIGINT UNSIGNED AUTO_INCREMENT` | 主键。 |
| `name` | `VARCHAR(255)` | 卡组名称；不要求唯一。 |
| `source_text` | `LONGTEXT` | 最近一次成功解析并保存的编辑器文本。 |
| `version` | `BIGINT UNSIGNED` | 乐观锁版本，初始为 1。 |
| `created_at` | `DATETIME(6)` | 创建时间。 |
| `updated_at` | `DATETIME(6)` | 名称或内容最近修改时间。 |

索引：`(updated_at, id)`。卡组名称当前不提供独立检索条件，因此不为 `VARCHAR(255)` 名称建立索引，避免旧版 InnoDB 的 `utf8mb4` 索引长度差异。

保存流程先完整解析 `source_text`。只要存在无法识别、数量非法或名称歧义的行，就返回全部行号和错误，且不修改 `deck`、`deck_card` 或操作日志。解析全部成功后，才在同一事务中替换明细、递增 `version` 并写一条“修改卡组”日志。

### 4.5 `deck_card`：卡组逻辑卡牌明细

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `deck_id` | `BIGINT UNSIGNED` | 外键到 `deck(id)`，删除卡组时级联删除。 |
| `section_type` | `VARCHAR(32)` | 固定分区代码：`mainboard`、`sideboard`、`commander` 或 `maybeboard`。 |
| `oracle_id` | `CHAR(36)` | 逻辑卡牌 ID。 |
| `quantity` | `INT UNSIGNED` | 编辑数量，必须大于 0。 |
| `name_snapshot` | `VARCHAR(512)` | 保存时的标准组合牌名，用于 Card DB 暂时缺失时诊断和展示。 |

主键：`(deck_id, section_type, oracle_id)`。反向统计索引：`(oracle_id, deck_id, quantity)`；分区展示索引由主键覆盖。`section_type` 的四个合法值由应用层枚举校验，数据库不使用 `CHECK` 或 `ENUM`，便于兼容和后续迁移。

卡组不保存 Set、语言、普通/闪、展示版本或实体卡分配。同一逻辑卡牌可以出现在多个分区，但在同一分区中只能有一行。分区不是用户可自定义实体，名称由界面本地化，固定展示顺序为 `commander`、`mainboard`、`sideboard`、`maybeboard`。文本编辑器可以接受分区标题的本地化别名，但解析后必须规范化为上述代码；首个分区标题之前的卡牌行归入 `mainboard`，无法识别的标题必须报错。

### 4.6 `operation_log`：操作日志主表

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `id` | `BIGINT UNSIGNED AUTO_INCREMENT` | 主键。 |
| `action_type` | `TINYINT UNSIGNED` | 只允许下表中的 8 个代码。 |
| `operation_date` | `DATE` | 按应用部署时区计算的自然日。 |
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

索引：`(operation_date, action_type, occurred_at, id)`。查询仍可使用 `ORDER BY occurred_at DESC, id DESC`；索引定义不写 `DESC`，避免依赖数据库对降序索引的版本支持。

每日汇总直接 `GROUP BY operation_date, action_type` 计算，不单独维护汇总表。只有业务状态确实变化时才写日志；重复加入愿望单、数量增量为 0 或未改变内容的卡组保存不产生日志。

### 4.7 `operation_card_detail`：卡牌操作快照

| 列 | 类型 | 约束/说明 |
| --- | --- | --- |
| `operation_id` | `BIGINT UNSIGNED` | 主键；外键到 `operation_log(id)`。 |
| `scryfall_id` | `CHAR(36)` | 操作时的印刷版本 ID。 |
| `oracle_id` | `CHAR(36) NULL` | 操作时的逻辑卡牌 ID。 |
| `card_name` | `VARCHAR(512)` | 操作时按说明语言回退规则得到的组合牌名快照。 |
| `set_code` | `VARCHAR(32)` | 系列代码快照。 |
| `collector_number` | `VARCHAR(64)` | 收藏编号快照。 |
| `lang` | `VARCHAR(16)` | 实体牌语言快照。 |
| `finish_kind` | `TINYINT UNSIGNED NULL` | 收藏操作为 1=普通、2=闪；愿望单操作为 `NULL`。 |
| `quantity_delta` | `BIGINT NULL` | 收藏操作保存有符号变化量；使用 `BIGINT` 才能覆盖 `INT UNSIGNED` 数量列的完整变化范围。愿望单操作为 `NULL`。 |

日志必须保存展示快照，不能依赖以后仍能从 Card DB 查到同一记录。增加/减少数量时，一次请求对一个印刷版本和一种工艺产生一条日志；例如一次增加 3 张普通卡记录 `quantity_delta=3`，不拆成三条。

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

| 合并 Set | 合并语言 | 分组键 |
| --- | --- | --- |
| 否 | 否 | `scryfall_id` |
| 是 | 否 | `COALESCE(oracle_id, scryfall_id), lang` |
| 否 | 是 | `COALESCE(oracle_id, scryfall_id), set_id, collector_number` |
| 是 | 是 | `COALESCE(oracle_id, scryfall_id)` |

普通和闪卡分别 `SUM`。展示信息、筛选和最终排序再由 Card DB 按这一页涉及的 ID 批量取得。由于两个数据库可能跨主机，不设计跨库 SQL JOIN。

### 5.2 卡组拥有量与异常

按 `oracle_id` 汇总收藏两列之和得到实际拥有量。计算当前卡组数量和所有卡组编辑总数时，只汇总 `section_type IN ('mainboard', 'sideboard', 'commander')` 的 `deck_card.quantity`；同一张牌出现在一副卡组的多个计数分区时，使用卡组数仍只计算一次 `COUNT(DISTINCT deck_id)`。备选区只展示，不产生数量不足或多卡组占用。当前卡组数量不足优先于多卡组占用，判断公式直接采用需求文档第 11 节，不落库存分配表。

类型、颜色、Mana Value、曲线以及土地/非地统计从 Card DB 为每个 `oracle_id` 选择一份逻辑卡牌属性后计算。卡组列表的总数、异常数和统计均实时派生；单用户规模下无需维护容易失真的缓存列。

### 5.3 卡组自动展示版本

候选层级固定为收藏版本、愿望单版本、其他最新印刷版本。每层内使用确定性顺序：

1. `released_at` 较新优先；
2. 语言代码、`set_code`、收藏编号和 `scryfall_id` 作为稳定的最终排序键。

选择结果不写入 App DB，每次展示时计算，因此收藏和愿望单改变后会立即生效，也不会被误解为实体卡分配。

## 6. 写入事务和一致性

- 调整收藏：锁定 `collection_item`，校验新数量，写入或删除收藏行，并写操作日志；全部在同一 App DB 事务中完成。
- 加入/移出愿望单：状态变化和日志在同一事务中完成。
- 创建、复制、修改、删除卡组：`deck`、`deck_card` 和一条日志在同一事务中完成。修改使用 `WHERE id=? AND version=?` 防止两个页面互相覆盖。
- `card_print_ref` 可在同一事务中 `INSERT ... ON DUPLICATE KEY UPDATE last_verified_at=...`；不能用新快照静默改变已有 `scryfall_id` 的身份字段，发现不一致时应报数据错误。
- 操作日志是追加数据，不提供普通业务删除或修改接口。若以后增加日志保留策略，应整条删除主表及其明细，不能留下半条事件。

## 7. 暂不建立的结构

- `users`、`app_profile`、角色和权限表：当前没有认证且一个实例只有一个所有者；页面名称读取 `app.display_name` 配置。
- 每张实体卡一行的实例表：需求明确只记录汇总数量。
- 库存分配表：卡组占用只是统计，不代表实体牌分配。
- 卡组展示版本列：展示版本由实时规则自动选择。
- 日志每日汇总表：可从有覆盖索引的明细实时聚合。
- 价格、交易、品相、合法性、愿望数量和愿望备注相关表：均不在当前范围。

## 8. 迁移落地顺序

首批 Goose migration 按以下顺序各自创建一张表：`card_print_ref`、`collection_item`、`wishlist_item`、`deck`、`deck_card`、`operation_log`、`operation_card_detail`、`operation_deck_detail`。MySQL DDL 会隐式提交，因此不把多张表放进同一个版本；失败后可以从尚未完成的版本继续重试。回滚时按版本反序删除。

迁移 SQL 统一位于 `migrations/app`，由 `migrations/embed.go` 内嵌到可执行文件。服务连接 Card DB 和 App DB 后、启动 HTTP 服务前自动升级 App DB；迁移失败时关闭数据库连接并终止启动。Goose 使用 App DB 中的 `goose_db_version` 表记录已执行版本。Card DB 不由 Goose 管理。

迁移中只对 App DB 内部关系使用具名外键，不使用 `CHECK`。数量总和、分区代码、动作代码、工艺代码以及操作明细与动作类型的匹配关系均由应用层集中校验；数值列继续使用 `UNSIGNED`、`NOT NULL` 和事务作为数据库层保护。

兼容性基线还包括：不使用原生 `JSON`、生成列、表达式索引、降序索引和依赖版本的 `DEFAULT` 表达式；所有时间与初始值由应用显式写入。App DB 的复合主键和普通索引在 `utf8mb4` 下均低于旧版 InnoDB 常见的 767 字节索引键限制。
