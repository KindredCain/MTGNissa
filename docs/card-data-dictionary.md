# 卡牌数据字典

## 1. 文档范围

本文描述当前导入器使用的九个 NDJSON/JSONL 文件字段，其中包含 Scryfall 官方 `rulings.jsonl` 和 `oracle-tags.jsonl`。

- `scryfall_card.json`：以 Scryfall Card Object 文档为主要依据。
- `rulings.jsonl`：以 Scryfall Ruling Object 为依据。
- `oracle-tags.jsonl`：以 Scryfall Tags API 文档为依据。
- 其余 `zhs_*.json`：以 MTGCH OpenAPI 的 `components.schemas` 为主要依据。
- 字段清单仅来自每个文件的第一条记录，没有扫描全部数据。
- `类型` 中的 `?` 表示字段可能为 `null`。
- NDJSON 文件每一行都是一个独立 JSON 对象，不是一个完整 JSON 数组。

### 1.1 反斜杠导入与展示规则

- 原有七个 `scryfall_card.json`/`zhs_*.json` 文件存在同一层导出问题：每个反斜杠被额外复制一次，导入器会在 JSON 校验和解析前撤销这一层。
- Scryfall 官方 `rulings.jsonl` 是标准 JSONL，不执行上述兼容性反斜杠处理。
- Scryfall 官方 `oracle-tags.jsonl` 同样是标准 JSONL，不执行兼容性反斜杠处理。
- 撤销导出层后，标准 JSON 解析器继续处理 JSON 转义并将解析结果按原义入库；不再对整个字符串做通用反转义。
- 展示 `zhs_*` 文本时，只将字面量 `\n` 转换为实际换行符。不额外处理字面量 `\r\n`，不删除单独的反斜杠，也不对引号或其他反斜杠序列做二次转义。
- 例如 `USG/163s` 在入库后的逻辑值为“一个字面量反斜杠 + 实际 CRLF”；展示层必须保留该反斜杠。

### 1.2 可信度

| 标记 | 含义 |
| --- | --- |
| 官方 | 字段名称和含义可由对应 API 文档直接确认。 |
| 映射 | 本地字段由官方字段重命名、单值化或展开得到。 |
| Schema | 字段可在 MTGCH `components.schemas` 的相关响应对象中确认。 |
| 对应概念 | 官方文档存在语义相近的输入或展示字段，但没有定义本地导出列。 |
| 样本确认 | 由用户提供的具体卡牌记录及页面行为确认。 |
| 部分确认 | 已确认基本类型或主要用途，但枚举、边界或普遍性仍不完整。 |
| 推定 | 文档没有直接定义原始导出字段，含义根据字段名、样本值和响应对象推定。 |
| 未确认 | 仅能确认字段和值类型，无法从指定文档确定完整语义或枚举。 |

### 1.3 重新评估原则

- Scryfall Card Object 文档定义的是 Scryfall API 字段，不直接定义 `zhs_*.json`。
- MTGCH `components.schemas` 定义的是 API 聚合响应，也不直接定义六个 `zhs_*` 导出对象。
- 只有 Schema 名称、类型和业务位置都一致时才标记为 `Schema`；仅仅出现同名字段不能证明二者相同。
- `对应概念` 表示可以找到 Scryfall 或 MTGCH 的展示字段，但本地字段仍经过拆分、翻译或重命名。
- 用户提供的 SOI #209、VOW #344、FIC #334 样本用于确认 `zhs_card.json` 的实际行为，结论不扩展到其他文件的同名字段。

## 2. 文件关系

数据模型以 `scryfall_card` 为主：每个卡牌及其具体印刷版本都必须来自 `scryfall_card`。六个 `zhs_*` 文件仅作为简体中文翻译、本地化展示字段或辅助词典，不定义独立卡牌实体。业务查询应从 `scryfall_card` 出发左连接需要的 `zhs_*` 数据；没有可用中文值时使用 Scryfall 字段。

### 2.1 字典、位掩码与检索关联

颜色、语言、布局、卡框、卡框效果、工艺和游戏平台是小型说明性字典。字典的代码及英文说明来自 Scryfall 官方 Colors、Languages、Layouts、Frames 和 Card Object 元数据：

| 字典表 | 卡牌表关联 |
| --- | --- |
| `scryfall_color` | `colors_mask`、`color_identity_mask`、`color_indicator_mask` 使用字典中的 `bit_value`。 |
| `scryfall_language` | `scryfall_card.lang` 保存语言代码。 |
| `scryfall_layout` | `scryfall_card.layout` 保存布局代码。 |
| `scryfall_frame` | `scryfall_card.frame` 保存卡框代码。 |
| `scryfall_frame_effect` | `scryfall_card_frame_effect.frame_effect` 保存卡框效果代码。 |
| `scryfall_finish` | `scryfall_card.finishes_mask` 使用字典中的 `bit_value`。 |
| `scryfall_game` | `scryfall_card.games_mask` 使用字典中的 `bit_value`。 |

位掩码定义为：颜色 `W=1`、`U=2`、`B=4`、`R=8`、`G=16`、`C=32`；工艺 `nonfoil=1`、`foil=2`、`etched=4`；平台 `paper=1`、`arena=2`、`mtgo=4`、`astral=8`、`sega=16`。其中 `astral` 和 `sega` 是 Scryfall 卡牌记录中实际存在的历史数字游戏平台，分别用于 Astral Cards 和 Sega Dreamcast Cards。Attraction 灯号 1–6 分别使用第 0–5 位。

`produced_mana` 不属于颜色字典关联，也不建立关联表。导入时仅校验源值为字符串数组，随后保持数组顺序并以逗号连接，直接保存到主表的可空 `VARCHAR(64)` 列。例如 `["W","U","B","R","G"]` 保存为 `W,U,B,R,G`，`["T"]` 保存为 `T`；不限制元素值域。

取值范围较大或需要保留数量/顺序的属性仍使用关联表：

| 关联表 | 来源字段 | 用途 |
| --- | --- | --- |
| `scryfall_card_type` | `type_line` | 按长破折号分为 `main` 和 `subtype` 类型词。 |
| `scryfall_card_keyword` | `keywords` | 按关键字异能检索。 |
| `scryfall_card_frame_effect` | `frame_effects` | 按卡框效果检索。 |
| `scryfall_card_promo_type` | `promo_types` | 按推广版本类型检索。 |
| `scryfall_oracle_ruling` | `rulings.jsonl` | 保存 Oracle 身份与规则内容之间的多对多关联、来源和发布日期。 |
| `oracle_tagging` | `oracle-tags.jsonl` | 保存 Oracle 身份与功能标签之间的多对多关联及关联强度。 |

`scryfall_set` 从卡牌文件中的 `set_id`、`set_code`、`set_name` 和 `set_type` 动态汇总，卡牌主表仅保留 `set_id`。`preview` 不参与检索，以规范 JSON 文本存入主表的可空 `LONGTEXT` 列。Oracle Tag 只导入本系统需要的名称、说明、别名、层级与卡牌关联，不保留 Tagger URI 等外部站点展示信息。

`artist_ids` 不用于当前检索，按源顺序以逗号连接保存在 `scryfall_card`；`former_names` 可能包含逗号，以压缩 JSON 数组文本保存在 `zhs_oracle`。两者只作为元数据保留，不拆关联表、不建立索引。

系统不提供单个费用符号查询，因此不从 `mana_cost` 派生费用符号表。完整费用展示和精确值保留在 `mana_cost`，费用高低及范围查询使用 `cmc`，卡牌颜色和颜色标识查询使用对应位掩码。

主表为 `cmc`、`mana_cost`、牌名、稀有度、发行日期、语言、布局、卡框、位掩码和印刷版本关联字段建立 B-tree 索引，并为英文牌名、单面牌名、`type_line` 和 `oracle_text` 建立全文索引。

目前可以使用或推定的主要关联如下：

| 主文件字段 | 关联文件字段 | 状态 | 用途 |
| --- | --- | --- | --- |
| `scryfall_card.face_oracle_id` | `zhs_oracle.face_oracle_id` | 已确认唯一 | 关联某一个牌面的 Oracle 中文翻译。 |
| `scryfall_card.flavor_id` | `zhs_flavor.flavor_id` | 已确认唯一 | 关联特定印刷牌面的背景叙述翻译。 |
| `scryfall_card.set_id` | `scryfall_set.set_id` | 已确认唯一 | 关联由 Scryfall 卡牌数据汇总得到的英文系列实体。 |
| `scryfall_set.set_id` | `zhs_set.set_id` | 已确认唯一 | 关联 MTGCH `SetSchema.id` 对应的系列中文翻译。 |
| `scryfall_card.uuid` | `zhs_card.card_id` | 已确认唯一 | 关联特定印刷及语言版本的中文印刷文字。 |
| `scryfall_card.oracle_id` | `zhs_oracle.oracle_id` | 官方/Schema | 将同一 Oracle 身份的不同印刷版本归组。 |
| `scryfall_card.multiverse_id` | `zhs_card.multiverse_id` | 映射 | 辅助关联 Gatherer 中文印刷数据。 |
| `scryfall_card.oracle_id` | `scryfall_oracle_ruling.oracle_id` | 官方 | 查找同一 Oracle 身份的全部单卡释疑。 |
| `scryfall_oracle_ruling.ruling_key` | `zhs_ruling.ruling_key` | 导入派生 | 查找规则英文内容及可选中文翻译。 |
| `scryfall_card.oracle_id` | `oracle_tagging.oracle_id` | 官方 | 查找逻辑卡牌的功能、机制及主题标签。 |

`zhs_ruling.json` 自带的 `ruling` 仅作为来源记录 ID 保存为 `ruling_id`，不再承担卡牌关联。跨数据源关联使用英文规则正文规范化后计算的 `ruling_key`。

### 2.2 数据库物理模型

本节记录导入器当前实际创建的正式表。所有表使用 `InnoDB`、`utf8mb4` 和 `utf8mb4_unicode_ci`。除明确标记为 `NULL` 的列外均为 `NOT NULL`；当前列均未声明数据库默认值。`BOOLEAN` 使用 MySQL 布尔类型语法，实际等价于 `TINYINT(1)`。

为了支持整套表通过一次 `RENAME TABLE` 原子切换，当前不创建跨表物理外键。下文“关联”均为由导入校验和业务查询维护的逻辑外键；主键、普通索引和全文索引仍会实际创建。

#### 2.2.1 `scryfall_card` 卡牌印刷主表

一行代表一个具体语言、版本和牌面的 Scryfall 卡牌记录，主键为 `uuid`。

| 列 | MySQL 类型 | 来源/用途 |
| --- | --- | --- |
| `uuid` | `CHAR(36)` | 主键；本地卡牌记录标识。 |
| `scryfall_id` | `CHAR(36)` | Scryfall 卡牌对象 ID。 |
| `face_index` | `INT` | 多面牌牌面序号，非牌面记录通常为 `-1`。 |
| `lang` | `VARCHAR(16)` | 语言代码，逻辑关联 `scryfall_language.code`。 |
| `oracle_id` | `CHAR(36) NULL` | Oracle 身份。 |
| `layout` | `VARCHAR(64)` | 布局代码，逻辑关联 `scryfall_layout.code`。 |
| `arena_id` | `BIGINT NULL` | Arena ID。 |
| `mtgo_id` | `BIGINT NULL` | MTGO ID。 |
| `mtgo_foil_id` | `BIGINT NULL` | MTGO 闪卡 ID。 |
| `multiverse_id` | `BIGINT NULL` | Gatherer Multiverse ID。 |
| `tcgplayer_id` | `BIGINT NULL` | TCGplayer ID。 |
| `tcgplayer_etched_id` | `BIGINT NULL` | TCGplayer 蚀刻闪 ID。 |
| `cardmarket_id` | `BIGINT NULL` | Cardmarket ID。 |
| `resource_id` | `VARCHAR(255) NULL` | 上游资源标识。 |
| `cmc` | `DECIMAL(12,4)` | 法术力值，支持范围检索。 |
| `colors_mask` | `TINYINT UNSIGNED` | 由 `colors` 生成的颜色位掩码。 |
| `color_identity_mask` | `TINYINT UNSIGNED` | 由 `color_identity` 生成的颜色标识位掩码。 |
| `color_indicator_mask` | `TINYINT UNSIGNED` | 由 `color_indicator` 生成的颜色指示位掩码。 |
| `produced_mana` | `VARCHAR(64) NULL` | 可产法术力或特殊资源代码；源字符串数组按原顺序以逗号连接，不使用字典或关联表。 |
| `defense` | `VARCHAR(32) NULL` | 战役牌防御力。 |
| `game_changer` | `BOOLEAN` | 是否为 Game Changer。 |
| `hand_modifier` | `VARCHAR(32) NULL` | Vanguard 手牌修正。 |
| `life_modifier` | `VARCHAR(32) NULL` | Vanguard 生命修正。 |
| `loyalty` | `VARCHAR(32) NULL` | 忠诚值，保留非纯数字表达。 |
| `name` | `VARCHAR(512)` | 英文牌名。 |
| `face_name` | `VARCHAR(512) NULL` | 当前牌面名称。 |
| `oracle_text` | `LONGTEXT NULL` | Oracle 规则叙述。 |
| `power` | `VARCHAR(32) NULL` | 力量，保留 `*` 等表达。 |
| `reserved` | `BOOLEAN` | 是否属于保留列表。 |
| `toughness` | `VARCHAR(32) NULL` | 防御力，保留 `*` 等表达。 |
| `type_line` | `VARCHAR(512)` | 完整英文类别栏。 |
| `artist` | `VARCHAR(255) NULL` | 展示用画师名称。 |
| `artist_ids` | `VARCHAR(2048) NULL` | Scryfall 画师 UUID 数组按源顺序以逗号连接；不参与检索。 |
| `booster` | `BOOLEAN` | 是否可能出现在补充包中。 |
| `border_color` | `VARCHAR(32)` | 边框颜色。 |
| `card_back_id` | `CHAR(36) NULL` | 卡背 ID。 |
| `collector_number` | `VARCHAR(64)` | 收藏编号。 |
| `content_warning` | `BOOLEAN NULL` | 内容警告标志。 |
| `digital` | `BOOLEAN` | 是否仅为数字版本。 |
| `finishes_mask` | `TINYINT UNSIGNED` | 由 `finishes` 生成的工艺位掩码。 |
| `flavor_name` | `VARCHAR(512) NULL` | 异画牌名。 |
| `flavor_text` | `LONGTEXT NULL` | 风味文字。 |
| `frame` | `VARCHAR(32)` | 卡框代码，逻辑关联 `scryfall_frame.code`。 |
| `full_art` | `BOOLEAN` | 是否为全图版本。 |
| `games_mask` | `TINYINT UNSIGNED` | 由 `games` 生成的平台位掩码。 |
| `highres_image` | `BOOLEAN` | 是否具有高分辨率图像。 |
| `illustration_id` | `CHAR(36) NULL` | 插画 ID。 |
| `image_status` | `VARCHAR(32)` | 图像状态。 |
| `oversized` | `BOOLEAN` | 是否为大尺寸卡。 |
| `printed_name` | `VARCHAR(512) NULL` | 该语言的印刷牌名。 |
| `printed_text` | `LONGTEXT NULL` | 该语言的印刷规则叙述。 |
| `printed_type_line` | `VARCHAR(512) NULL` | 该语言的印刷类别栏。 |
| `promo` | `BOOLEAN` | 是否为推广版本。 |
| `rarity` | `VARCHAR(32)` | 稀有度。 |
| `released_at` | `DATE` | 发行日期。 |
| `reprint` | `BOOLEAN` | 是否为重印。 |
| `set_id` | `CHAR(36)` | 逻辑关联 `scryfall_set.set_id`。 |
| `story_spotlight` | `BOOLEAN` | 是否为故事焦点牌。 |
| `textless` | `BOOLEAN` | 是否为无字版本。 |
| `variation` | `BOOLEAN` | 是否为另一个印刷版本的变体。 |
| `variation_of` | `CHAR(36) NULL` | 被变体记录的 Scryfall ID。 |
| `security_stamp` | `VARCHAR(64) NULL` | 防伪标记。 |
| `watermark` | `VARCHAR(128) NULL` | 水印。 |
| `foil` | `BOOLEAN` | 上游兼容字段：是否存在传统闪。 |
| `nonfoil` | `BOOLEAN` | 上游兼容字段：是否存在普通不闪。 |
| `attraction_lights_mask` | `TINYINT UNSIGNED` | 由 `attraction_lights` 生成的灯号位掩码。 |
| `preview` | `LONGTEXT NULL` | 不参与检索的 `preview` 对象；以压缩后的合法 JSON 文本保存。 |
| `mana_cost` | `VARCHAR(255) NULL` | 完整法术力费用；不拆分符号，直接用于展示或完整值查询。 |
| `flavor_id` | `CHAR(36) NULL` | 逻辑关联 `zhs_flavor.flavor_id`。 |
| `face_oracle_id` | `CHAR(36) NULL` | 逻辑关联 `zhs_oracle.face_oracle_id`。 |
| `created_at` | `DATETIME(6)` | 上游创建时间。 |
| `updated_at` | `DATETIME(6)` | 上游更新时间。 |

普通索引：`scryfall_id`、`oracle_id`、`face_oracle_id`、`flavor_id`、`set_id`、`multiverse_id`、`(set_id, collector_number, lang)`、`cmc`、`mana_cost`、`rarity`、`released_at`、`name`、`lang`、`layout`、`frame`、`colors_mask`、`color_identity_mask`、`finishes_mask`、`games_mask`。

全文索引：`(name, face_name, type_line, oracle_text)`。

#### 2.2.2 系列实体与中文系列翻译

| 表 | 列 | 主键 | 索引及关联 |
| --- | --- | --- | --- |
| `scryfall_set` | `set_id CHAR(36) NOT NULL`、`code VARCHAR(32) NOT NULL`、`name VARCHAR(255) NOT NULL`、`set_type VARCHAR(64) NOT NULL` | `set_id` | 索引 `code`、`set_type`；从卡牌源字段动态汇总。 |
| `zhs_set` | `set_id CHAR(36) NOT NULL`、`code VARCHAR(32) NULL`、`name VARCHAR(255) NULL`、`source VARCHAR(128) NULL`、`stage INT NULL` | `set_id` | 索引 `code`；`set_id` 逻辑关联 `scryfall_set.set_id`，只提供中文翻译元数据。 |

系列查询的标准关联链为：

```text
scryfall_card.set_id
    -> scryfall_set.set_id
    -> zhs_set.set_id
```

展示名称优先使用非空的 `zhs_set.name`，否则回退 `scryfall_set.name`。不得使用 `zhs_set` 创建卡牌或英文系列实体。

#### 2.2.3 中文翻译和本地化表

| 表 | 列定义 | 主键 | 普通索引/逻辑关联 |
| --- | --- | --- | --- |
| `zhs_card` | `card_id CHAR(36) NOT NULL`、`name VARCHAR(512) NULL`、`face_name VARCHAR(512) NULL`、`flavor_name VARCHAR(512) NULL`、`type_line VARCHAR(512) NULL`、`text LONGTEXT NULL`、`flavor_text LONGTEXT NULL`、`multiverse_id BIGINT NULL`、`source VARCHAR(128) NULL`、`extra LONGTEXT NULL` | `card_id` | 索引 `multiverse_id`；`card_id` 逻辑关联 `scryfall_card.uuid`。 |
| `zhs_flavor` | `flavor_id CHAR(36) NOT NULL`、`name VARCHAR(512) NULL`、`flavor_name VARCHAR(512) NULL`、`flavor_text LONGTEXT NULL`、`set VARCHAR(32) NULL`、`collector_number VARCHAR(64) NULL`、`released_at DATE NULL`、`translated_flavor_name VARCHAR(512) NULL`、`translated_flavor_text LONGTEXT NULL`、`flavor_updated_at DATE NULL`、`extra LONGTEXT NULL`、`name_source VARCHAR(128) NULL`、`name_stage INT NULL`、`text_source VARCHAR(128) NULL`、`text_stage INT NULL` | `flavor_id` | 组合索引 `(set, collector_number)`；由 `scryfall_card.flavor_id` 关联。 |
| `zhs_oracle` | `face_oracle_id CHAR(36) NOT NULL`、`oracle_id CHAR(36) NULL`、`name VARCHAR(512) NULL`、`set VARCHAR(32) NULL`、`collector_number VARCHAR(64) NULL`、`released_at DATE NULL`、`type_line VARCHAR(512) NULL`、`oracle_text LONGTEXT NULL`、`translated_name VARCHAR(512) NULL`、`name_stage INT NULL`、`name_source VARCHAR(128) NULL`、`translated_type VARCHAR(512) NULL`、`type_stage INT NULL`、`translated_text LONGTEXT NULL`、`text_stage INT NULL`、`text_source VARCHAR(128) NULL`、`extra LONGTEXT NULL`、`former_names LONGTEXT NULL` | `face_oracle_id` | 索引 `oracle_id`、`(set, collector_number)`；由 `scryfall_card.face_oracle_id` 关联。`former_names` 保存压缩 JSON 数组文本。 |
| `zhs_ruling` | `ruling_key CHAR(64) NOT NULL`、`ruling_id CHAR(36) NULL`、`comment LONGTEXT NOT NULL`、`translation LONGTEXT NULL`、`translation_source VARCHAR(128) NULL`、`translation_stage INT NULL`、`last_published_at DATE NULL`、`extra LONGTEXT NULL` | `ruling_key` | 索引 `ruling_id`；保存去重后的规则正文和可选中文翻译。 |
| `scryfall_oracle_ruling` | `id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT`、`oracle_id CHAR(36) NOT NULL`、`ruling_key CHAR(64) NOT NULL`、`source VARCHAR(32) NOT NULL`、`published_at DATE NOT NULL` | `id` | 索引 `(oracle_id, published_at)`、`(ruling_key, oracle_id)`；每条 Scryfall 源记录独立保存。 |
| `zhs_type` | `type_name VARCHAR(255) NOT NULL`、`type_type VARCHAR(64) NOT NULL`、`translation VARCHAR(512) NULL`、`stage INT NULL`、`created_at DATETIME(6) NULL`、`is_funny BOOLEAN NULL` | `(type_name, type_type)` | 中文类别词翻译数据，不定义卡牌实体。 |

`zhs_ruling.extra` 接受对象、数组、字符串、数字、布尔值或 `null`；非空值压缩为合法 JSON 文本保存到普通 `LONGTEXT`，不拆分子表且不参与检索。其他本地化表的 `extra` 仍按各自既有标量定义保存。

#### 2.2.4 多值属性和检索派生表

| 表 | 列定义 | 主键 | 检索索引 | 来源与逻辑关联 |
| --- | --- | --- | --- | --- |
| `scryfall_card_keyword` | `card_uuid CHAR(36) NOT NULL`、`keyword VARCHAR(255) NOT NULL` | `(card_uuid, keyword)` | `(keyword, card_uuid)` | `keywords[]`；`card_uuid -> scryfall_card.uuid`。 |
| `scryfall_card_type` | `card_uuid CHAR(36) NOT NULL`、`type_group VARCHAR(16) NOT NULL`、`type_name VARCHAR(128) NOT NULL` | `(card_uuid, type_group, type_name)` | `(type_group, type_name, card_uuid)` | 从 `type_line` 派生；`type_group` 当前为 `main` 或 `subtype`。 |
| `scryfall_card_frame_effect` | `card_uuid CHAR(36) NOT NULL`、`frame_effect VARCHAR(64) NOT NULL` | `(card_uuid, frame_effect)` | `(frame_effect, card_uuid)` | `frame_effects[]`；效果代码逻辑关联 `scryfall_frame_effect.code`。 |
| `scryfall_card_promo_type` | `card_uuid CHAR(36) NOT NULL`、`promo_type VARCHAR(128) NOT NULL` | `(card_uuid, promo_type)` | `(promo_type, card_uuid)` | `promo_types[]`。 |
| `oracle_tag_relation` | `parent_tag_id CHAR(36) NOT NULL`、`child_tag_id CHAR(36) NOT NULL` | `(parent_tag_id, child_tag_id)` | `(child_tag_id, parent_tag_id)` | 标签父子关系；两个字段均逻辑关联 `oracle_tag.tag_id`。 |
| `oracle_tagging` | `oracle_id CHAR(36) NOT NULL`、`tag_id CHAR(36) NOT NULL`、`weight VARCHAR(16) NOT NULL` | `(oracle_id, tag_id)` | `(tag_id, weight, oracle_id)` | Oracle 卡牌与标签关联；`oracle_id -> scryfall_card.oracle_id`，`tag_id -> oracle_tag.tag_id`。 |

上表的 `card_uuid` 均逻辑关联 `scryfall_card.uuid`。这些表只保存集合成员或派生检索项；用于展示的原始 `mana_cost`、`type_line` 和 `artist` 仍保留在主表。

`oracle_tag` 主表定义如下：

| 列 | MySQL 类型 | 用途 |
| --- | --- | --- |
| `tag_id` | `CHAR(36)` | 主键；来源标签 UUID。 |
| `label` | `VARCHAR(255)` | 标签显示及检索名称，普通索引。 |
| `description` | `LONGTEXT NULL` | 社区维护的英文标签说明，允许为空。 |
| `aliases` | `VARCHAR(1024) NULL` | 别名数组按源顺序以逗号连接，不单独建表。 |

#### 2.2.5 小型说明字典表

| 表 | 列定义 | 主键 | 使用位置 |
| --- | --- | --- | --- |
| `scryfall_color` | `code CHAR(1) NOT NULL`、`bit_value TINYINT UNSIGNED NOT NULL`、`name VARCHAR(32) NOT NULL`、`description VARCHAR(255) NOT NULL`、`sort_order TINYINT UNSIGNED NOT NULL` | `code` | 三个颜色相关位掩码。 |
| `scryfall_language` | `code VARCHAR(8) NOT NULL`、`name VARCHAR(64) NOT NULL` | `code` | `scryfall_card.lang`。 |
| `scryfall_layout` | `code VARCHAR(64) NOT NULL`、`name VARCHAR(128) NOT NULL`、`description VARCHAR(512) NOT NULL`、`face_category VARCHAR(32) NOT NULL` | `code` | `scryfall_card.layout`。 |
| `scryfall_frame` | `code VARCHAR(32) NOT NULL`、`name VARCHAR(128) NOT NULL`、`description VARCHAR(512) NOT NULL` | `code` | `scryfall_card.frame`。 |
| `scryfall_frame_effect` | `code VARCHAR(64) NOT NULL`、`description VARCHAR(512) NOT NULL` | `code` | `scryfall_card_frame_effect.frame_effect`。 |
| `scryfall_finish` | `code VARCHAR(32) NOT NULL`、`bit_value TINYINT UNSIGNED NOT NULL`、`name VARCHAR(64) NOT NULL` | `code` | `scryfall_card.finishes_mask`。 |
| `scryfall_game` | `code VARCHAR(32) NOT NULL`、`bit_value TINYINT UNSIGNED NOT NULL`、`name VARCHAR(64) NOT NULL` | `code` | `scryfall_card.games_mask`。 |

字典元数据由导入器在每次加载时写入临时字典表并随其他表一起切换，不从卡牌记录反向推测。卡牌、系列和多值关联数据仍全部来自导入文件。

#### 2.2.6 位掩码查询约定

检查“包含某值”时使用按位与，而不是直接等值比较。例如查询颜色标识中包含蓝色（`U=2`）：

```sql
SELECT *
FROM scryfall_card
WHERE (color_identity_mask & 2) = 2;
```

查询恰好为蓝色则使用 `color_identity_mask = 2`。无色或空集合使用 `0`。导入器遇到尚未登记的颜色、工艺、游戏平台或非法 Attraction 灯号时会终止校验，避免静默丢失信息。

## 3. `scryfall_card.json`

该文件是经过扁平化和重命名的 Scryfall Card Object，并非 Scryfall 原始响应。

### 3.1 标识与结构

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `uuid` | UUID | 未确认 | 当前数据集内部的卡牌记录主键；不是 Scryfall 标准 `id` 字段。 |
| `scryfall_id` | UUID | 映射 | Scryfall Card Object 的 `id`，唯一标识一个具体印刷版本。 |
| `face_index` | integer | 已确认/映射 | 扁平化后的 Scryfall `card_faces` 数组下标：没有 `card_faces` 的普通单面牌为 `-1`，首个牌面为 `0`，第二个牌面为 `1`。图片侧面还需结合 `layout`：`split`、`flip`、`adventure` 的多个牌面都在 `front`；`transform`、`modal_dfc`、`double_faced_token`、`art_series` 及 `reversible_card` 的 `0` 为正面、`1` 为背面。 |
| `lang` | string | 官方 | 当前印刷版本的语言代码。 |
| `oracle_id` | UUID? | 官方 | Oracle 身份 ID；同一规则身份的重印版本通常共享此值。 |
| `layout` | string | 官方 | 卡牌版面类型，例如 `normal`、`split`、`transform`、`modal_dfc`。 |
| `face_oracle_id` | UUID? | Schema | MTGCH 使用的牌面级 Oracle 关联 ID；业务仅按等值关联使用，将其视为不透明标识，不依赖生成规则。 |
| `created_at` | datetime | 未确认 | 当前数据集记录的创建时间，不是 Scryfall Card Object 字段。 |
| `updated_at` | datetime | 未确认 | 当前数据集记录的更新时间，不是 Scryfall Card Object 字段。 |

### 3.2 外部平台标识

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `arena_id` | integer? | 官方 | Arena 卡牌 ID。 |
| `mtgo_id` | integer? | 官方 | Magic Online 普通版本 Catalog ID。 |
| `mtgo_foil_id` | integer? | 官方 | Magic Online 闪卡版本 Catalog ID。 |
| `multiverse_id` | integer? | 映射 | Scryfall `multiverse_ids` 数组的单值化结果，关联 Gatherer。 |
| `tcgplayer_id` | integer? | 官方 | TCGplayer 普通商品 ID。 |
| `tcgplayer_etched_id` | integer? | 官方 | TCGplayer 蚀刻闪商品 ID。 |
| `cardmarket_id` | integer? | 官方 | Cardmarket 商品 ID。 |
| `resource_id` | string? | 官方 | Gatherer Resource ID。 |

### 3.3 游戏属性

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `cmc` | number | 官方 | Mana Value，旧称 converted mana cost。 |
| `color_identity` | string[] | 官方 | 卡牌的颜色标识。 |
| `color_indicator` | string[]? | 官方 | 颜色指示符中的颜色。 |
| `colors` | string[]? | 官方 | 卡牌或当前牌面的颜色。 |
| `defense` | string? | 官方 | 战役牌的防御力。 |
| `game_changer` | boolean | 官方 | Scryfall 标记的 Commander Game Changer。 |
| `hand_modifier` | string? | 官方 | Vanguard 等牌的起手牌数量修正。 |
| `keywords` | string[] | 官方 | 卡牌拥有的关键字异能名称。 |
| `life_modifier` | string? | 官方 | Vanguard 等牌的起始生命修正。 |
| `loyalty` | string? | 官方 | 旅法师的初始忠诚值。 |
| `mana_cost` | string? | 官方 | 使用 `{G}`、`{2/U}` 等符号表示的法术力费用。 |
| `name` | string | 官方 | Scryfall 卡牌名称；多面牌可能是组合名称。 |
| `face_name` | string? | 映射 | 扁平化后的单独牌面名称。 |
| `oracle_text` | string? | 官方 | 英文 Oracle 规则叙述。 |
| `power` | string? | 官方 | 生物力量，使用字符串以支持 `*` 等非数字值。 |
| `produced_mana` | string[]? | 官方 | 卡牌能够产生的法术力或特殊资源代码；关系模型中以逗号连接字符串保存。 |
| `reserved` | boolean | 官方 | 是否属于 Reserved List。 |
| `toughness` | string? | 官方 | 生物防御力，使用字符串以支持 `*` 等非数字值。 |
| `type_line` | string | 官方 | 英文类别栏。 |

### 3.4 印刷与美术属性

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `artist` | string? | 官方 | 插画作者名称。 |
| `artist_ids` | UUID[]? | 官方 | Scryfall 插画作者 ID；关系模型中按源顺序以逗号连接保存在主表，不单独建表。 |
| `attraction_lights` | integer[]? | 官方 | Attraction 牌上点亮的数字。 |
| `booster` | boolean | 官方 | 该印刷版本是否可能出现在补充包中。 |
| `border_color` | string | 官方 | 卡牌边框颜色。 |
| `card_back_id` | UUID? | 官方 | 对应卡背图案的 Scryfall ID。 |
| `collector_number` | string | 官方 | 系列内收藏编号；可能包含字母等非数字字符。 |
| `content_warning` | boolean? | 官方 | Scryfall 是否为该卡标记敏感内容警告。 |
| `digital` | boolean | 官方 | 是否仅存在于数字平台。 |
| `finishes` | string[] | 官方 | 可用工艺，例如 `nonfoil`、`foil`、`etched`。数据本身保持原值。 |
| `flavor_name` | string? | 官方 | Showcase 等版本印在卡面上的替代名称。 |
| `flavor_text` | string? | 官方 | 当前印刷版本的英文背景叙述。 |
| `flavor_id` | UUID? | Schema | MTGCH 背景叙述翻译关联 ID。 |
| `frame_effects` | string[]? | 官方 | 卡框附加效果，例如 legendary crown、showcase。 |
| `frame` | string | 官方 | 卡框年代或样式标识。 |
| `full_art` | boolean | 官方 | 是否为全图版本。 |
| `games` | string[] | 官方 | 可使用该印刷版本的平台，例如 `paper`、`arena`、`mtgo`。 |
| `highres_image` | boolean | 官方 | Scryfall 是否提供高分辨率图片。 |
| `illustration_id` | UUID? | 官方 | 插画身份 ID；相同插画通常共享此值。 |
| `image_status` | string | 官方 | 图片状态，例如 `missing`、`placeholder`、`lowres`、`highres_scan`。 |
| `oversized` | boolean | 官方 | 是否为超大尺寸印刷。 |
| `printed_name` | string? | 官方 | 当前语言实际印刷的牌名。 |
| `printed_text` | string? | 官方 | 当前语言实际印刷的规则叙述。 |
| `printed_type_line` | string? | 官方 | 当前语言实际印刷的类别栏。 |
| `promo` | boolean | 官方 | 是否为赠卡或推广版本。 |
| `promo_types` | string[]? | 官方 | 推广版本类型。 |
| `rarity` | string | 官方 | 稀有度。 |
| `released_at` | date | 官方 | 该印刷版本的发行日期。 |
| `reprint` | boolean | 官方 | 是否为重印版本。 |
| `security_stamp` | string? | 官方 | 防伪标记类型。 |
| `story_spotlight` | boolean | 官方 | 是否为剧情焦点牌。 |
| `textless` | boolean | 官方 | 是否为无文字版本。 |
| `variation` | boolean | 官方 | 是否为另一印刷版本的变体。 |
| `variation_of` | UUID? | 官方 | 被变体版本所指向的基础 Scryfall 卡牌 ID。 |
| `watermark` | string? | 官方 | 卡面水印。 |

### 3.5 系列属性

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `set_code` | string | 映射 | Scryfall `set`，系列代码。当前样本使用大写，Scryfall 通常返回小写。 |
| `set_id` | UUID | 官方 | Scryfall 系列对象 ID。 |
| `set_name` | string | 官方 | 系列英文名称。 |
| `set_type` | string | 官方 | 系列类型，例如 `core`、`expansion`、`commander`。 |

### 3.6 工艺可用性兼容字段

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `foil` | boolean | 官方 | 该印刷版本是否存在传统闪工艺。 |
| `nonfoil` | boolean | 官方 | 该印刷版本是否存在普通不闪工艺。 |
| `preview` | object? | 官方 | 剧透来源、来源 URI 和预览日期等信息。 |

计数业务中仅做归类映射：`nonfoil` 计入普通数量，`foil` 和 `etched` 计入闪卡数量；不得修改这里的原始工艺数据。

### 3.7 未保留的关键 Scryfall 字段

当前头部记录未包含以下常用 Scryfall 字段：

- `image_uris`：卡图地址；当前方案可由后端根据 `scryfall_id` 生成 CDN 地址。
- `card_faces`：多面牌对象；当前数据看起来已通过 `face_index`、`face_name` 等字段展开。
- `legalities`：赛制合法性；本项目已明确不做合法性校验。
- `prices`：价格；本项目已明确不做价格功能。
- `all_parts`、`related_uris`、`purchase_uris` 及多个 Scryfall 页面/API URI。

## 4. `zhs_card.json`

该文件保存特定印刷/语言版本的简体中文文字。MTGCH Schemas 没有同名原始对象，API 中相关内容会被合并到 `CardViewSchema` 和 `CardFaceSchema`。Scryfall 中最接近的概念是本地化卡牌对象的 `printed_name`、`printed_text`、`printed_type_line`、`flavor_name` 与 `flavor_text`，但字段并非逐项原样复制。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `card_id` | UUID | 已确认唯一 | 中文印刷记录的唯一关联键，关联 `scryfall_card.uuid`。不是 MTGCH 收藏功能中的 `card_id` 语义。 |
| `name` | string? | 样本确认/对应概念 | 标准卡牌身份的简体中文名称；即使印刷版本具有替代牌名，这里仍保存标准牌名。多面牌使用“正面 // 背面”的组合名称。未提供中文资料的占位记录中允许为 `null`，此时通过 `card_id` 关联 `scryfall_card` 并回退英文。API 聚合后接近 `CardFaceSchema.name_zhs`。 |
| `face_name` | string? | 样本确认/本地展开 | 当前记录所对应牌面的标准简体中文名称；多面牌的牌面记录使用该字段，单面牌通常为 `null`。Scryfall 和 MTGCH Schemas 均没有这个原始列名。 |
| `flavor_name` | string? | 样本确认/对应概念 | 当前印刷版本实际展示的简体中文替代牌名，例如 Dracula 系列牌的“约翰西沃德医生”；没有替代牌名时为 `null`。对应 Scryfall `flavor_name` 概念和 `CardFaceSchema.flavor_name_zhs`。 |
| `type_line` | string? | 样本确认/对应概念 | 当前记录所对应牌面的简体中文印刷类别栏。接近 Scryfall `printed_type_line`，API 聚合后对应 `CardFaceSchema.type_line_zhs`。 |
| `text` | string? | 样本确认/对应概念 | 当前记录所对应牌面的简体中文印刷规则叙述，不合并多面牌其他牌面的文字。接近 Scryfall `printed_text`，API 聚合后对应 `CardFaceSchema.oracle_text_zhs_html`。 |
| `flavor_text` | string? | 样本确认/对应概念 | 当前记录所对应牌面的简体中文印刷背景叙述。对应 Scryfall `flavor_text` 概念和 `CardFaceSchema.flavor_text_zhs_html`。 |
| `multiverse_id` | integer? | 样本确认/映射 | 当前简体中文印刷版本的 Gatherer Multiverse ID；由 Scryfall `multiverse_ids` 单值化，API 聚合后对应 `CardViewSchema.zhs_multiverse_id`。允许为 `null`，缺少该 ID 不代表中文内容不存在。 |
| `source` | string? | 样本部分确认 | 当前中文印刷数据的可选来源元信息；`Chinese Simplified` 是已观察值。指定文档没有定义此列，它不是可靠的语言字段，也不是各文本字段的翻译者来源；完整语义和枚举仍未确认。 |
| `extra` | string? | 样本确认/Schema 映射 | 当前中文印刷记录的补充展示文字，例如 `FINAL FANTASY X` 用于标明关联作品；API 聚合后对应 `CardViewSchema.zhs_extra`。允许为 `null`，其他可能内容尚未枚举。 |

## 5. `zhs_flavor.json`

该文件保存特定印刷版本背景叙述及其中文翻译状态。Scryfall Card Object 提供原始 `name`、`flavor_name`、`flavor_text`、`set`、`collector_number` 和 `released_at` 概念；MTGCH API 最终主要映射到 `CardFaceSchema.flavor_text_zhs_html`、`flavor_name_zhs` 和 `TranslationSourceSchema`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `flavor_id` | UUID | 已确认唯一 | 背景叙述记录的唯一关联键，对应 `CardFaceSchema.flavor_id`。 |
| `name` | string | 已确认/复用规则 | 标准英文卡牌名称；在具有替代牌名的印刷版本中仍保存标准名称。多面牌复用 `zhs_card.name` 的规则，保存“正面 // 背面”的组合名称。 |
| `flavor_name` | string? | 样本确认/Scryfall 对应概念 | 当前印刷版本的英文替代牌名；例如 `Mothra, Supersonic Queen`。没有替代牌名时允许为 `null`。 |
| `flavor_text` | string? | 已确认/Scryfall 对应概念 | 具体印刷版本的英文背景叙述原文；没有背景叙述时允许为 `null`。 |
| `set` | string | Scryfall/Schema 对应概念 | 系列代码。 |
| `collector_number` | string | Scryfall/Schema 对应概念 | 收藏编号。 |
| `released_at` | date | Scryfall/Schema 对应概念 | 该印刷版本发行日期。 |
| `translated_flavor_name` | string? | 样本确认/对应概念 | `flavor_name` 的简体中文翻译；例如“超音速女王摩斯拉”。API 聚合后对应 `CardFaceSchema.flavor_name_zhs`。 |
| `translated_flavor_text` | string? | 对应概念 | 简体中文背景叙述翻译；API 聚合后对应 `CardFaceSchema.flavor_text_zhs_html`。原始列名未出现在 Schemas 中。 |
| `flavor_updated_at` | date? | 已明确为非业务字段 | 当前背景叙述/替代牌名翻译记录的更新时间信息。系统只按可空日期读取，不依赖其具体触发条件，也不用于查询、排序或回退判断。 |
| `extra` | string? | 已确认/复用规则 | 复用 `zhs_card.extra` 的结构和用途，为可空的补充展示文字；不属于牌名、规则叙述或背景叙述正文。 |
| `name_source` | string? | 样本确认/Schema 对应概念 | `translated_flavor_name` 的翻译来源；样本值包括 `MTGZH`。聚合后对应 `TranslationSourceSchema.name_source`，完整枚举仍未确认。 |
| `name_stage` | integer | 部分确认 | 替代牌名翻译流程阶段；已观察到有翻译时为 `5`、缺失时可能为 `0`，但完整数值枚举和状态定义仍未确认。 |
| `text_source` | string? | 样本确认/Schema 映射 | `translated_flavor_text` 的翻译来源；没有对应翻译时可以为 `null`。API 聚合后映射为 `TranslationSourceSchema.flavor_source`。 |
| `text_stage` | integer | 部分确认 | 背景叙述翻译流程阶段；已观察到缺少原文和翻译时为 `0`，但完整数值枚举和状态定义仍未确认。 |

## 6. `zhs_oracle.json`

该文件保存牌面级 Oracle 原文及简体中文翻译。Scryfall Card Object 定义 `oracle_id`、`name`、`type_line`、`oracle_text`、`set`、`collector_number` 和 `released_at`；MTGCH API 最终主要映射到 `CardFaceSchema` 的 `*_atomic`、`*_zhs` 字段。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `face_oracle_id` | UUID | 已确认唯一 | MTGCH 牌面级 Oracle 唯一关联键，对应 `CardFaceSchema.face_oracle_id`；业务仅用它关联 `scryfall_card.face_oracle_id`，将其视为不透明标识，不关心生成规则。 |
| `oracle_id` | UUID? | Scryfall/Schema | Scryfall Oracle 身份 ID，用于跨印刷版本归组。 |
| `name` | string | Scryfall 对应概念 | 原始牌面名称；导出文件是否只允许英文仍需样本确认。 |
| `set` | string | Scryfall/Schema 对应概念 | 用于确定当前翻译记录来源的系列代码。 |
| `collector_number` | string | Scryfall/Schema 对应概念 | 用于确定当前翻译记录来源的收藏编号。 |
| `released_at` | date | Scryfall/Schema 对应概念 | 来源印刷版本的发行日期。 |
| `type_line` | string? | Scryfall 对应概念 | 原始 Oracle 类别栏；导出文件是否只允许英文仍需确认。 |
| `oracle_text` | string? | Scryfall 对应概念 | 原始 Oracle 规则叙述；导出文件是否只允许英文仍需确认。 |
| `translated_name` | string? | 映射 | 简体中文牌名；API 中对应 `CardFaceSchema.name_zhs`。 |
| `name_stage` | integer | 未确认 | 牌名翻译流程阶段，数值枚举未定义。 |
| `name_source` | string? | Schema 对应概念 | 牌名翻译来源，聚合后对应 `TranslationSourceSchema.name_source`。 |
| `translated_type` | string? | 映射 | 简体中文类别栏；API 中对应 `CardFaceSchema.type_line_zhs`。 |
| `type_stage` | integer | 未确认 | 类别栏翻译流程阶段，数值枚举未定义。 |
| `translated_text` | string? | 映射 | 简体中文规则叙述；API 中对应 `CardFaceSchema.oracle_text_zhs_html` 的文本来源。 |
| `text_stage` | integer | 未确认 | 规则叙述翻译流程阶段，数值枚举未定义。 |
| `text_source` | string? | Schema 对应概念 | 规则叙述翻译来源，聚合后对应 `TranslationSourceSchema.text_source`。 |
| `former_names` | array | Schema | 曾用中文名称列表；以压缩 JSON 数组文本保存在 `zhs_oracle.former_names`，保留名称边界和顺序。 |
| `extra` | unknown? | 未确认 | 扩展信息，结构未在 Schemas 中定义。 |

## 7. `zhs_ruling.json`

该文件保存原始裁定及简体中文翻译。导入后与 `rulings.jsonl` 共同形成 `zhs_ruling` 内容表；已有中文行优先，Scryfall 数据不会覆盖翻译。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `ruling` | UUID | 未确认 | 裁定来源记录内部 ID；入库列名为 `ruling_id`，仅供追溯，不用于关联卡牌。 |
| `comment` | string | Schema | 英文裁定正文；必须非空，用于计算 `ruling_key`。 |
| `translation` | string? | Schema | 简体中文裁定翻译。 |
| `source` | string? | Schema | 翻译来源；入库列名为 `translation_source`。 |
| `stage` | integer | 未确认 | 翻译流程阶段；入库列名为 `translation_stage`。 |
| `last_published_at` | date? | Schema 映射 | 最近发布日期；API 聚合后字段名为 `RulingSchema.published_at`。 |
| `extra` | JSON? | 样本确认 | 扩展信息；对象等任意合法 JSON 值压缩后保存为文本，允许 `null`。 |

`ruling_key` 不直接来自文件。生成规则是：将 `comment` 的 CRLF/CR 统一为 LF，将弯单引号、弯双引号和不换行空格统一为普通字符，去除首尾空白，再计算小写十六进制 SHA-256。不会折叠正文内部普通空白，也不会翻译或改写文本。

## 8. `rulings.jsonl`

该文件来自 Scryfall Bulk Data 的 Rulings 导出。一条记录表示某个 Oracle 身份关联的一条裁定；同一规则正文可以关联多个 `oracle_id`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `object` | string | 官方 | 固定为 `ruling`；其他值拒绝导入。 |
| `oracle_id` | UUID | 官方 | 卡牌 Oracle 身份，逻辑关联 `scryfall_card.oracle_id`。 |
| `source` | string | 官方 | 裁定来源，例如 `wotc`。 |
| `published_at` | date | 官方 | 该 Oracle 身份下裁定的发布日期。 |
| `comment` | string | 官方 | 英文裁定正文，用相同规范化算法生成 `ruling_key`。 |

每条记录都独立写入 `scryfall_oracle_ruling` 并取得自增 `id`。同一 `oracle_id`、同一正文即使只因发布日期不同而重复出现，也不覆盖或合并；`id` 只是当前全量加载结果中的内部行标识，加载后可以变化。内容表仍按 `ruling_key` 向 `zhs_ruling` 合并：已有中文记录时仅取较新的 `last_published_at`，保留全部翻译字段；没有内容记录时新增只有英文 `comment` 和日期的行，`ruling_id`、`translation`、翻译来源、翻译阶段及 `extra` 均为 `NULL`。

标准查询链为：

```text
scryfall_card.oracle_id
    -> scryfall_oracle_ruling.oracle_id
    -> zhs_ruling.ruling_key
```

## 9. `oracle-tags.jsonl`

该文件来自 Scryfall Tagger 社区维护的 Oracle Tags 数据。Oracle 标签描述卡牌规则功能、机制、用途或主题，关联的是逻辑卡牌 `oracle_id`，而不是具体印刷版本。系统不复刻 Scryfall Tagger 页面，只导入卡牌查询需要的信息。

| 字段 | 类型 | 入库规则 |
| --- | --- | --- |
| `object` | string | 必须为 `tag`，仅用于校验，不入库。 |
| `id` | UUID | 写入 `oracle_tag.tag_id`，作为标签稳定主键。 |
| `label` | string | 写入 `oracle_tag.label`，用于标签展示和检索。 |
| `slug` | string | Scryfall URL 标识，本系统不使用，不入库。 |
| `type` | string | 当前文件必须为 `oracle`，仅用于拒绝误放的 illustration 标签，不入库。 |
| `uri` | string | Scryfall Tagger 页面地址，本系统不使用，不入库。 |
| `description` | string? | 写入 `oracle_tag.description`，允许为 `NULL`，文本原样保存。 |
| `parent_ids` | UUID[] | 每个父标签生成 `(parent_id, 当前标签ID)` 关系。 |
| `child_ids` | UUID[] | 每个子标签生成 `(当前标签ID, child_id)` 关系。 |
| `aliases` | string[] | 按源顺序以逗号连接写入 `oracle_tag.aliases`；不建立别名表。 |
| `taggings` | object[] | 展开写入 `oracle_tagging`。 |
| `taggings[].oracle_id` | UUID | 逻辑关联 `scryfall_card.oracle_id`。 |
| `taggings[].weight` | string | 标签与卡牌的关联强度，原样保存到 `VARCHAR(16)`；不建立字典且不限制枚举。 |

`parent_ids` 与其他记录的 `child_ids` 可能表达同一条边。导入器统一转换为 `(parent_tag_id, child_tag_id)`，在整个文件范围按联合主键去重，不保存两套方向相反的关系。

标准卡牌标签查询链为：

```text
scryfall_card.oracle_id
    -> oracle_tagging.oracle_id
    -> oracle_tag.tag_id
```

## 10. `zhs_set.json`

该文件保存系列简体中文名称。Scryfall Card Object 的印刷字段包含 `set_id`、`set`、`set_name`；MTGCH API 最终将中文名称合并进 `SetSchema.translated_name`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `set_id` | UUID | 已确认唯一 | 系列唯一关联键，对应 Scryfall `set_id`、`scryfall_card.set_id` 与 `SetSchema.id`。 |
| `code` | string | Scryfall/Schema 映射 | 系列代码，对应 Scryfall `set` 与 `SetSchema.code`。 |
| `name` | string? | Schema 映射 | 系列简体中文名称；API 中对应 `SetSchema.translated_name`，而非英文 `SetSchema.name`。 |
| `source` | string? | 未确认 | 系列名称翻译来源，枚举未定义。 |
| `stage` | integer | 未确认 | 系列名称翻译流程阶段，数值枚举未定义。 |

## 11. `zhs_type.json`

该文件保存卡牌类别词汇翻译。Scryfall Card Object 只提供完成后的 `type_line`，MTGCH Schemas 也没有公开类别词典的原始或响应对象，因此本文件所有字段都无法从指定文档直接确认。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `type_name` | string | 推定 | 英文类别词汇。 |
| `type_type` | string | 推定 | 词汇类别，例如样本中的 `Type`；完整枚举未定义。 |
| `translation` | string? | 样本确认 | 简体中文翻译。尚未翻译的词条允许为 `null`，例如 `stage` 为 `0` 的 `Coming Soon!`。 |
| `stage` | integer | 未确认 | 翻译流程阶段，数值枚举未定义。 |
| `created_at` | datetime | 推定 | 词汇翻译记录创建时间。 |
| `is_funny` | boolean | 推定 | 是否属于非正规或趣味牌相关词汇。 |

## 12. 对当前业务的关键字段

| 业务需求 | 建议字段 |
| --- | --- |
| 区分具体印刷版本 | `scryfall_id`；数据库内部也可继续使用 `uuid`。 |
| 跨系列合并同一张牌 | `oracle_id`；多面牌显示还需结合 `face_oracle_id` 和 `face_index`。 |
| 系列内定位 | `set_code` + `collector_number`，必要时再加 `lang`。 |
| 区分实体牌语言 | `lang`，不能用中文翻译是否存在来代替。 |
| 普通/闪卡可用工艺 | `finishes`、`foil`、`nonfoil`。 |
| 卡牌中文名称和规则叙述 | 优先使用 `zhs_oracle` 中非空的翻译字段，不检查 `stage`；翻译缺失时回退 `scryfall_card` 英文 Oracle 字段。 |
| 中文缺失判断 | `null`、空字符串和仅包含空白字符的字符串统一视为缺失。 |
| 特定印刷中文文字 | `zhs_card`，通过本地卡牌 ID 或 `multiverse_id` 关联。 |
| 背景叙述中文翻译 | `flavor_id` 关联 `zhs_flavor`。 |
| 系列中文名称 | `set_id` 关联 `zhs_set`。 |
| 单卡释疑 | 由卡牌 `oracle_id` 关联 `scryfall_oracle_ruling`，再按 `ruling_key` 读取 `zhs_ruling`；中文为空时显示 `comment`。 |
| Oracle 功能标签 | 由卡牌 `oracle_id` 关联 `oracle_tagging`，再按 `tag_id` 读取 `oracle_tag`；可使用 `weight` 排序相关性。 |
| 卡图 | `scryfall_id`、`face_index`、`layout`；由后端集中生成 Scryfall CDN 地址。 |
| 排序 | `set_code`、`collector_number`、`colors`、`color_identity`、`cmc`、`name`。 |

## 13. 尚需确认

这些文件是只读卡牌元数据，服务不会增删改，也不会生成其中的 ID 或维护翻译流程。因此只需要确认会影响查询结果、关联结果或语言回退的规则。

### 13.1 实现前需要确认

当前没有阻塞只读查询、关联、中文展示或英文回退实现的待确认项。

中文字段的统一处理规则已经确定：先去除首尾空白，结果为空或原值为 `null` 时视为缺失，并回退对应英文内容。

### 13.2 对应功能启用时再确认

- 如果名称搜索需要兼容历史译名，再确认 `zhs_oracle.former_names` 的元素结构和匹配规则。
- 如果需要展示 `extra`，再确认 `zhs_oracle.extra`、`zhs_ruling.extra` 等字段的结构；当前需求可以忽略。
- 如果需要使用 `zhs_type` 动态翻译类别词汇，再确认 `type_type`、`stage` 和 `is_funny`；当前可以直接使用卡牌记录中已经生成的中文类别栏。

### 13.3 不需要确认

- UUID 的生成算法。
- `source` 的完整枚举，除非界面要展示翻译来源。
- `stage` 的完整枚举及具体含义；运行时不读取 `stage`，所有翻译字段非空时直接展示。
- `created_at`、`updated_at`、`flavor_updated_at` 的触发条件。
- 单个 `multiverse_id` 从上游数组中的选择算法；服务只把现有值作为可空外部标识读取。
- `extra` 的完整内容范围，除非产品明确要求展示。
- 原文字段是否全部来自同一种语言；英文回退可以直接读取 Scryfall 标准字段，不依赖 `zhs_*` 中的原文字段。

## 14. 参考文档

- [Scryfall Card Objects](https://scryfall.com/docs/api/cards)
- [Scryfall Ruling Objects](https://scryfall.com/docs/api/rulings)
- [Scryfall Tags](https://scryfall.com/docs/api/tags)
- [Scryfall Card Images](https://scryfall.com/docs/api/images)
- [MTGCH API 文档](https://mtgch.com/api/v1/docs)
- [MTGCH OpenAPI Schema](https://mtgch.com/api/v1/openapi.json)
