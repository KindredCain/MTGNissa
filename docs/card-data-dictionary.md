# 卡牌数据字典

本文描述项目的**目标数据模型**，只保留项目实际设计和导入后需要持久化的表；不把上游 JSON 的原始对象、历史 SQLite 结构或废弃的 `rulings.jsonl` 作为项目表记录。

## 1. 数据源与通用约定

| 数据源 | 用途 |
| --- | --- |
| `AllPrintings.json` | 唯一的卡牌元数据来源；JSON 文件格式，提供英文印刷卡、Token、系列、裁定、赛制合法性及大部分枚举值。 |
| `zhs_card.json` | 简体中文卡牌、中文译名与中文印刷文字。 |
| `zhs_flavor.json` | 简体中文风味文字。 |
| `zhs_oracle.json` | 简体中文 Oracle（规则文字）翻译。 |
| `zhs_set.json` | 简体中文系列名称。 |
| `zhs_type.json` | 类型词翻译。 |
| `zhs_ruling.json` | 简体中文裁定翻译。 |
| `oracle-tags.jsonl` | Oracle 标签、标签层级及 Oracle 标签标注。 |
| `scryfall_card.json` | 不读取卡牌元数据；仅作为旧卡牌 UUID 与 `scryfall_id` 的只读映射桥，用于将既有翻译关联重写为新的 AllPrintings 卡面 UUID。 |
| 项目内置字典 | 颜色、语言、版式等稳定枚举；不依赖导入文件。 |

### 数据文件实际结构

- `AllPrintings.json` 是一个标准 JSON 文档，顶层包含 `meta` 和 `data`；`data` 是以系列代码为键、系列对象为值的对象，不是 `sets` 数组。导入时遍历 `data` 的所有属性值，并校验属性键与系列对象内的 `code` 一致。本文为简化字段来源而使用的 `sets[]` 表示 `AllPrintings.json: data` 的系列对象集合；`sets[].cards[]`、`sets[].tokens[]` 分别表示每个系列对象内的 `cards[]`、`tokens[]`。
- `scryfall_card.json`、`zhs_card.json`、`zhs_flavor.json`、`zhs_oracle.json`、`zhs_ruling.json`、`zhs_set.json` 和 `zhs_type.json` 虽使用 `.json` 扩展名，实际均为 NDJSON：每个非空物理行是一个独立 JSON 对象，文件整体不是 JSON 数组或单个 JSON 对象。本文对这些文件使用的 `cards[]`、`flavors[]`、`oracles[]`、`rulings[]`、`sets[]`、`types[]` 仅表示对应文件的逐行记录集合。
- 上述旧 NDJSON 文件存在由上游重复添加的一层反斜杠转义。每行解析前必须先移除这一层重复转义，再按标准 JSON 对象解析；只能移除一层，不得继续反复解码文本内容。规范化后仍无法解析、不是 JSON 对象或缺少该文件必填标识字段的行必须报错。
- `oracle-tags.jsonl` 是标准 JSONL，每个非空物理行是一个独立 JSON 对象；不得应用旧 NDJSON 的反斜杠转义规范化。
- 已废弃的 `rulings.jsonl` 不属于导入文件；英文裁定只读取 `AllPrintings.json` 各卡牌的 `rulings[]`。
- 所有文件按 UTF-8 读取。大文件必须按其实际结构流式处理，不得要求将完整 `AllPrintings.json` 或整份 NDJSON 文件一次性载入内存。

- `uuid` 均为 UUID 字符串，使用 `CHAR(36)`。
- `uuid` 是本次 AllPrintings 导入生成的项目印刷卡面主键，来自 MTGJSON `uuid`；`scryfall_id` 是印刷卡面公开 ID，来自 `identifiers.scryfallId`。两者不可混用。
- 位掩码字段使用 `BIGINT UNSIGNED`；数据库不使用 `JSON` 列类型。上游数组按字段语义转换为位掩码、逗号连接字符串或关联表；必须保留的结构化原值压缩为 `LONGTEXT`。
- 卡牌、Token、系列、裁定和赛制元数据只从 `AllPrintings.json` 读取；不得从 `scryfall_card.json` 读取或补充元数据。
- 导入器必须先检查每个 `sets[]` 对象的 `isPartialPreview`：仅当该字段为 `false` 或未提供时才导入。字段为 `true` 的预览中 Set 整体跳过，并按 `parentCode` 递归跳过其所有子系列；不写入这些系列及其 `cards[]`、`tokens[]`、语言印刷、裁定、赛制合法性、字段展开或由 Token 派生出的 Token Set；不得以不完整数据补齐、合并或保留旧数据。
- `NULL` 表示上游未提供或该属性不适用，不能混同于空数组、空字符串或业务上的否定状态。
- AllPrintings 中映射到 `BOOLEAN` 列的真值标记采用缺省为假的规则：属性存在且为 `true` 时写入 `true`，属性缺失、为 `null` 或显式为 `false` 时均写入 `false`。该规则同样适用于 `hasContentWarning`、`hasAlternativeDeckLimit`，以及 `leadershipSkills` 对象和其中的 `brawl`、`commander`、`oathbreaker` 成员；对象或成员缺失时相应列写入 `false`。由数组推导的布尔列按数组是否包含对应值计算，数组缺失或为空时写入 `false`。
- AllPrintings 中普通印刷卡读取 `sets[].cards[]`，Token 读取 `sets[].tokens[]`。Token 不继承系列语言，也不进入赛制合法性表。
- AllPrintings 的语言名称在写入前必须转换为项目内置 `scryfall_language.code`：`English→en`、`Spanish→es`、`French→fr`、`German→de`、`Italian→it`、`Portuguese (Brazil)→pt`、`Japanese→ja`、`Korean→ko`、`Russian→ru`、`Chinese Simplified→zhs`、`Chinese Traditional→zht`、`Hebrew→he`、`Latin→la`、`Ancient Greek→grc`、`Arabic→ar`、`Sanskrit→sa`、`Phyrexian→ph`、`Quenya→qya`、`Dwarvish→dw`。遇到未收录的语言名称必须报错，不得原样写入或静默舍弃。

## 2. MTGNissa 印刷数据源表

### `mtgnissa_card`

列数：59。

作用：保存普通印刷卡及双面卡的英文基础印刷信息；一张多面卡按卡面拆行，`uuid` 为卡面记录主键。

关键约束：`uuid` 为主键；`scryfall_id` 必填，但多面卡可共享同一值，唯一性由 `(scryfall_id, face_index)` 保证；`set_code` 必须存在于 `mtgnissa_set.code`。默认展示、图片和翻译查询由本表的 `scryfall_id` 直接进入翻译链路；不因引入语言表而移除该字段。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| uuid | CHAR(36) | 卡面主键。 | `AllPrintings.json: cards[].uuid` |
| scryfall_id | CHAR(36) | 基础语言印刷卡面公开 ID。 | `AllPrintings.json: cards[].identifiers.scryfallId` |
| face_index | INT | 同一多面卡或多分栏牌中的卡面顺序；`side=a/b/c/d/e` 分别转换为 `0/1/2/3/4`。普通单面牌、没有 `side` 的记录和 Meld 独立牌为 `-1`。 | 按 `cards[].side` 与 `cards[].layout` 计算 |
| oracle_id | CHAR(36) NULL | Oracle 规则对象 ID。 | `cards[].identifiers.scryfallOracleId` |
| layout | VARCHAR(32) | 卡牌版式代码。 | `cards[].layout` |
| cmc | DECIMAL(12,4) | 当前卡面的法术力值；`faceManaValue` 属性存在时直接使用（包括合法值 `0`），仅在属性不存在时回退整卡 `manaValue`。 | `cards[].faceManaValue`，缺失时 `cards[].manaValue` |
| colors_mask | BIGINT UNSIGNED | 颜色位掩码。 | `cards[].colors` |
| color_identity_mask | BIGINT UNSIGNED | 颜色标识位掩码。 | `cards[].colorIdentity` |
| color_indicator_mask | BIGINT UNSIGNED | 颜色指示物位掩码。 | `cards[].colorIndicator` |
| produced_mana | VARCHAR(32) NULL | 可产生的法术力颜色集合。 | `cards[].producedMana` |
| defense | VARCHAR(32) NULL | 战斗甲虫等的防御力。 | `cards[].defense` |
| game_changer | BOOLEAN | 是否为 Game Changer。 | `cards[].isGameChanger` |
| hand_modifier | VARCHAR(32) NULL | 起始手牌修正。 | `cards[].hand` |
| life_modifier | VARCHAR(32) NULL | 起始生命修正。 | `cards[].life` |
| loyalty | VARCHAR(32) NULL | 忠诚度原文。 | `cards[].loyalty` |
| loyalty_numeric | DECIMAL(8,2) NULL | 可解析的忠诚度。 | 由 `cards[].loyalty` 解析 |
| name | VARCHAR(512) | 英文名称。 | `cards[].name` |
| face_name | VARCHAR(512) NULL | 多面卡单面名称。 | `cards[].faceName` |
| oracle_text | LONGTEXT NULL | 英文规则文字。 | `cards[].text` |
| power | VARCHAR(32) NULL | 力量原文。 | `cards[].power` |
| power_numeric | DECIMAL(8,2) NULL | 可解析的力量。 | 由 `cards[].power` 解析 |
| reserved | BOOLEAN | 是否在保留名单。 | `cards[].isReserved` |
| toughness | VARCHAR(32) NULL | 防御力原文。 | `cards[].toughness` |
| toughness_numeric | DECIMAL(8,2) NULL | 可解析的防御力。 | 由 `cards[].toughness` 解析 |
| pt_numeric | DECIMAL(12,4) NULL | 可解析力量与防御力之和；任一项不可解析时为空。 | 由 power、toughness 解析结果计算 |
| type_line | VARCHAR(512) | 英文类别行。 | `cards[].type` |
| artist | VARCHAR(512) NULL | 插画师显示名。 | `cards[].artist` |
| artist_ids | VARCHAR(2048) NULL | 按源顺序以逗号连接的插画师标识列表。 | `cards[].artistIds` |
| booster | BOOLEAN | 是否出现在任一补充包类型中。 | 由 `cards[].boosterTypes` 是否非空计算 |
| border_color | VARCHAR(32) | 边框颜色。 | `cards[].borderColor` |
| collector_number | VARCHAR(64) | 收藏家编号。 | `cards[].number` |
| content_warning | BOOLEAN | 是否有内容警告。 | `cards[].hasContentWarning` |
| digital | BOOLEAN | 是否仅数字版。 | `cards[].isOnlineOnly` |
| finishes_mask | BIGINT UNSIGNED | 可用闪处理位掩码。 | `cards[].finishes` |
| flavor_name | VARCHAR(512) NULL | 当前卡面的风味名称；优先卡面值，缺失时回退卡牌值。 | `cards[].faceFlavorName`，缺失时 `cards[].flavorName` |
| flavor_text | LONGTEXT NULL | 英文风味文字。 | `cards[].flavorText` |
| frame | VARCHAR(16) NULL | 卡框代码。 | `cards[].frameVersion` |
| full_art | BOOLEAN | 是否全画。 | `cards[].isFullArt` |
| games_mask | BIGINT UNSIGNED | 支持的游戏客户端位掩码。 | `cards[].availability` |
| oversized | BOOLEAN | 是否超大卡。 | `cards[].isOversized` |
| promo | BOOLEAN | 是否赠卡。 | `cards[].isPromo` |
| rarity | VARCHAR(32) | 稀有度。 | `cards[].rarity` |
| released_at | DATE NULL | 印刷发行日。 | `sets[].releaseDate` |
| reprint | BOOLEAN | 是否重印。 | `cards[].isReprint` |
| set_code | VARCHAR(16) | 所属系列代码。 | `cards[].setCode` |
| story_spotlight | BOOLEAN | 是否故事聚焦卡。 | `cards[].isStorySpotlight` |
| textless | BOOLEAN | 是否无文字卡。 | `cards[].isTextless` |
| variation | BOOLEAN | 是否变体印刷。 | `cards[].isAlternative` |
| variations | TEXT NULL | 按源顺序以逗号连接的变体 UUID 列表。 | `cards[].variations` |
| security_stamp | VARCHAR(64) NULL | 防伪印记。 | `cards[].securityStamp` |
| watermark | VARCHAR(128) NULL | 水印。 | `cards[].watermark` |
| foil | BOOLEAN | 是否有闪版本。 | 由 `cards[].finishes` 推导 |
| nonfoil | BOOLEAN | 是否有非闪版本。 | 由 `cards[].finishes` 推导 |
| attraction_lights_mask | BIGINT UNSIGNED | 游乐园灯光位掩码。 | `cards[].attractionLights` |
| mana_cost | VARCHAR(512) NULL | 法术力费用。 | `cards[].manaCost` |
| has_alternative_deck_limit | BOOLEAN | 是否有替代套牌张数限制。 | `cards[].hasAlternativeDeckLimit` |
| brawl_leader | BOOLEAN | 是否可作 Brawl 指挥官。 | `cards[].leadershipSkills.brawl` |
| commander_leader | BOOLEAN | 是否可作 Commander 指挥官。 | `cards[].leadershipSkills.commander` |
| oathbreaker_leader | BOOLEAN | 是否可作 Oathbreaker 誓约破坏者。 | `cards[].leadershipSkills.oathbreaker` |

建议索引：`oracle_id`、`set_code`、`name`、`released_at`，以及唯一索引 `(scryfall_id, face_index)`。

### `mtgnissa_token`

列数：38。

作用：保存 Token、徽记等 Token 类印刷卡面，与普通印刷卡物理分表；Token 仅进入 Token Set 卡牌列表。

关键约束：`uuid` 为主键；`set_code` 指向 Token 专用系列；`oracle_id` 和 `scryfall_id` 可为空；Token 不写入 `oracle_legality`，且不继承普通系列的语言列表。Token 不提供普通卡牌详情、收藏、愿望单、卡组、全局搜索或语言展示。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| uuid | CHAR(36) | Token 卡面主键。 | `AllPrintings.json: tokens[].uuid` |
| scryfall_id | CHAR(36) NULL | Token 印刷卡面公开 ID。 | `tokens[].identifiers.scryfallId` |
| face_index | INT | Token 多面顺序；`side=a/b/c/d/e` 分别转换为 `0/1/2/3/4`，没有 `side` 时为 `-1`。 | 按 `tokens[].side` 计算 |
| oracle_id | CHAR(36) NULL | Oracle 规则对象 ID。 | `tokens[].identifiers.scryfallOracleId` |
| layout | VARCHAR(32) | 版式代码。 | `tokens[].layout` |
| colors_mask | BIGINT UNSIGNED | 颜色位掩码。 | `tokens[].colors` |
| color_identity_mask | BIGINT UNSIGNED | 颜色标识位掩码。 | `tokens[].colorIdentity` |
| produced_mana | VARCHAR(32) NULL | 可产生的法术力。 | `tokens[].producedMana` |
| name | VARCHAR(512) | 英文名称。 | `tokens[].name` |
| face_name | VARCHAR(512) NULL | 卡面名称。 | `tokens[].faceName` |
| oracle_text | LONGTEXT NULL | 英文规则文字。 | `tokens[].text` |
| power | VARCHAR(32) NULL | 力量原文。 | `tokens[].power` |
| power_numeric | DECIMAL(8,2) NULL | 可解析力量。 | 由 `tokens[].power` 解析 |
| toughness | VARCHAR(32) NULL | 防御力原文。 | `tokens[].toughness` |
| toughness_numeric | DECIMAL(8,2) NULL | 可解析防御力。 | 由 `tokens[].toughness` 解析 |
| pt_numeric | DECIMAL(12,4) NULL | 可解析力量与防御力之和；任一项不可解析时为空。 | 由 power、toughness 解析结果计算 |
| type_line | VARCHAR(512) | 英文类别行。 | `tokens[].type` |
| artist | VARCHAR(512) NULL | 插画师。 | `tokens[].artist` |
| artist_ids | VARCHAR(2048) NULL | 按源顺序以逗号连接的插画师 ID 列表。 | `tokens[].artistIds` |
| booster | BOOLEAN | 是否出现在任一补充包类型中。 | 由 `tokens[].boosterTypes` 是否非空计算 |
| border_color | VARCHAR(32) | 边框颜色。 | `tokens[].borderColor` |
| collector_number | VARCHAR(64) | 收藏家编号。 | `tokens[].number` |
| finishes_mask | BIGINT UNSIGNED | 闪处理位掩码。 | `tokens[].finishes` |
| flavor_name | VARCHAR(512) NULL | Token 的风味名称。 | `tokens[].flavorName` |
| flavor_text | LONGTEXT NULL | 英文风味文字。 | `tokens[].flavorText` |
| frame | VARCHAR(16) NULL | 卡框代码。 | `tokens[].frameVersion` |
| full_art | BOOLEAN | 是否全画。 | `tokens[].isFullArt` |
| games_mask | BIGINT UNSIGNED | 游戏客户端位掩码。 | `tokens[].availability` |
| oversized | BOOLEAN | 是否超大卡。 | `tokens[].isOversized` |
| promo | BOOLEAN | 是否赠卡。 | `tokens[].isPromo` |
| reprint | BOOLEAN | 是否重印。 | `tokens[].isReprint` |
| set_code | VARCHAR(16) | Token 专用系列代码。 | 按 `tokenSetCode` 规则生成 |
| textless | BOOLEAN | 是否无文字。 | `tokens[].isTextless` |
| security_stamp | VARCHAR(64) NULL | 防伪印记。 | `tokens[].securityStamp` |
| watermark | VARCHAR(128) NULL | 水印。 | `tokens[].watermark` |
| foil | BOOLEAN | 是否有闪版本。 | 由 `tokens[].finishes` 推导 |
| nonfoil | BOOLEAN | 是否有非闪版本。 | 由 `tokens[].finishes` 推导 |
| mana_cost | VARCHAR(512) NULL | 法术力费用。 | `tokens[].manaCost` |

### `mtgnissa_set`

列数：9。

作用：保存普通系列和由 Token 派生出的 Token 专用系列。

关键约束：`code` 为主键；普通系列代码取 `sets[].code`；只要该系列有 Token，就额外生成一行 `set_type=token`。Token 系列代码在 `tokenSetCode` 非空且不同于主系列代码时取它，否则为 `T + code`；所有生成代码必须唯一。Token 系列的 `languages` 固定为 `NULL`，`parent_code` 指向来源普通系列；普通系列的 `parent_code` 保留 `sets[].parentCode`，用于表达系列层级和列表缩进。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(16) | 系列主键代码。 | 普通系列为 `sets[].code`；Token 系列按上述规则生成 |
| name | VARCHAR(512) | 英文系列名称。 | `sets[].name`；Token 系列由主系列名称派生 |
| released_at | DATE NULL | 发行日。 | `sets[].releaseDate` |
| set_type | VARCHAR(32) | 系列类别。 | 普通系列取 `sets[].type`；Token 固定为 `token` |
| card_count | BIGINT UNSIGNED | 该系列纳入的卡面数量。 | 普通系列由 `cards[]` 计数；Token 系列由 `tokens[]` 计数 |
| keyrune_code | VARCHAR(64) NULL | Keyrune 图标代码。 | `sets[].keyruneCode` |
| languages | VARCHAR(255) NULL | 按源顺序转换并以逗号连接的普通系列语言代码；Token 固定为 `NULL`。 | 普通系列取 `sets[].languages` 并转换为 `scryfall_language.code`；Token 为 `NULL` |
| block | VARCHAR(512) NULL | 所属 Block。 | `sets[].block` |
| parent_code | VARCHAR(16) NULL | 父系列代码，用于系列层级和列表缩进。 | 普通系列取 `sets[].parentCode`；Token 取来源外层 `sets[].code` |

### `mtgnissa_card_printing`

列数：7。

作用：保存印刷语言变体中真正需要随语言变化的三个文字字段，并保留该语言变体的印刷卡面 ID。

关键约束：`uuid` 为主键，`card_uuid` 外键指向 `mtgnissa_card.uuid`，`(card_uuid, language_code)` 唯一。语言名称必须先转换为项目内置 `scryfall_language.code`。同一卡牌的语言行以 `foreignData[]` 为优先来源：先按转换后的语言代码导入全部 `foreignData[]`；若 `foreignData[]` 已包含与外层 `cards[].language` 相同的语言，则不再新增基础语言行，并将该语言行的 `uuid` 改用外层 `cards[].uuid`，其 `scryfall_id` 和三个印刷文字字段仍取该 `foreignData[]`；若没有相同语言，才由外层卡牌补充基础语言行，此时 `uuid = card_uuid` 且 `scryfall_id = mtgnissa_card.scryfall_id`。同一卡牌的 `foreignData[]` 若在语言转换后仍产生重复代码，导入必须报错，不得静默覆盖。没有印刷文字时不创建无意义行；Token 不进入本表。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| uuid | CHAR(36) | 语言印刷记录主键；同语言冲突时保持与主卡一致。 | 一般变体取 `cards[].foreignData[].uuid`；与外层语言冲突或由外层补充时取 `cards[].uuid` |
| card_uuid | CHAR(36) | 所属 AllPrintings 主卡面。 | 外层 `cards[].uuid` |
| language_code | VARCHAR(8) | 项目语言代码。 | 将 `cards[].language` 或 `cards[].foreignData[].language` 转换为 `scryfall_language.code` |
| scryfall_id | CHAR(36) NULL | 该语言印刷卡面公开 ID。 | 优先取 `cards[].foreignData[].identifiers.scryfallId`；外层补充行取 `cards[].identifiers.scryfallId` |
| printed_name | VARCHAR(512) NULL | 该语言印刷名称。 | 优先取 `cards[].foreignData[].name`；外层补充行取 `cards[].printedName`，缺失时回退 `cards[].name` |
| printed_text | LONGTEXT NULL | 该语言印刷规则文字。 | 优先取 `cards[].foreignData[].text`；外层补充行取 `cards[].printedText`，缺失时回退 `cards[].text` |
| printed_type_line | VARCHAR(512) NULL | 该语言印刷类别行。 | 优先取 `cards[].foreignData[].type`；外层补充行取 `cards[].printedType`，缺失时回退 `cards[].type` |

建议索引：`card_uuid`、`scryfall_id`、`language_code`。

## 3. 翻译表

翻译表、既有翻译关联表及其物理字段、主键和翻译优先级均沿用原设计；本次变化仅发生在导入映射。`mtgnissa_card` 不新增任何翻译桥字段，继续仅保留本身已有的 `uuid` 与 `scryfall_id`。导入过程建立只在本次加载事务内存在的映射索引，不新增项目持久化表：

1. 从旧 `scryfall_card.json` 建立 `(scryfall_id, 卡面定位) -> 旧 uuid` 的只读索引。
2. 从 `AllPrintings.json` 分别建立 `(cards[].identifiers.scryfallId, 卡面定位) -> 新 cards[].uuid` 的普通卡索引和 `tokens[].identifiers.scryfallId -> tokens[].uuid` 的 Token 识别索引，并先写入新的 `mtgnissa_card` 与 `mtgnissa_token`。
3. 先按下述过滤规则排除 Token 和 `A-` 再平衡牌，再通过普通卡索引将原 `card_translation.card_id`、`card_flavor_translation_mapping.card_uuid` 和 `card_oracle_translation_mapping.card_uuid` 的关联值重写为新卡 UUID 后写入原表。

翻译映射过滤规则：

- 旧卡的 `scryfall_id` 目标位于 AllPrintings `tokens[]` 而非 `cards[]` 时，将其视为 Token 数据：不写入 `card_translation`，也不为该旧卡生成 `card_flavor_translation_mapping` 或 `card_oracle_translation_mapping`。不得仅按 `layout` 猜测 Token，应以 AllPrintings Token 识别索引的实际命中结果为准。
- 旧 `scryfall_card.json` 中英文 `name` 或卡面 `face_name` 去除首尾空白后以 `A-` 开头的炼金再平衡牌直接跳过，不写入卡牌翻译及两张翻译映射表。
- 因上述两类规则被跳过不属于导入错误。其 `flavor_translation`、`oracle_translation` 等独立翻译源记录仍可按各自主键导入，但不得为被跳过的卡生成卡牌关联。
- 除 Token 和 `A-` 再平衡牌外，其余旧卡若不能唯一映射到 `mtgnissa_card`，不得猜测或静默跳过；必须记录导入错误并保留旧 UUID、`scryfall_id`、卡面定位和失败原因等可追溯信息。

多面卡共享 `scryfall_id` 时，卡面定位必须参与匹配：统一以 `side=a/b/c/d/e` 对应 `face_index=0/1/2/3/4`，普通单面牌、没有 `side` 的记录和 Meld 独立牌使用 `face_index=-1`；必要时再以 `face_name` 等可验证字段核对。不得按单个 `scryfall_id` 覆盖映射。也就是说，翻译源表不以 AllPrintings UUID 作为源标识，但原翻译表及其两张关联表中写入 `card_id`、`card_uuid` 的关联值必须是新的 AllPrintings 卡面 UUID。

### `card_translation`

列数：10。

作用：保存中文卡牌基础翻译及可保留的源信息。

关键约束：表结构和字段语义保持既有设计，`card_id` 为主键并关联新的 `mtgnissa_card.uuid`。导入器不得直接把旧 `zhs_card.json` 的 `card_id` 写入：必须先应用统一的 Token 与 `A-` 再平衡牌过滤规则，再以旧 `scryfall_card.json.scryfall_id` 与 `AllPrintings.json: cards[].identifiers.scryfallId` 匹配，并将匹配到的 AllPrintings 卡面 UUID 写入本列。仅记录实际提供且成功唯一映射到普通卡的翻译；空字段不以空字符串覆盖已有数据。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_id | CHAR(36) | 新 `mtgnissa_card.uuid`，也是翻译记录主键。 | 导入映射：`zhs_card.json` 旧卡 UUID → 旧 `scryfall_card.scryfall_id` → AllPrintings 卡面 UUID |
| name | VARCHAR(512) NULL | 中文名称。 | `cards[].name` |
| face_name | VARCHAR(512) NULL | 中文卡面名称。 | `cards[].face_name` |
| flavor_name | VARCHAR(512) NULL | 中文风味名称。 | `cards[].flavor_name` |
| type_line | VARCHAR(512) NULL | 中文类别行。 | `cards[].type_line` |
| text | LONGTEXT NULL | 中文规则文字。 | `cards[].text` |
| flavor_text | LONGTEXT NULL | 中文风味文字。 | `cards[].flavor_text` |
| multiverse_id | BIGINT NULL | Multiverse 关联 ID。 | `zhs_card.json: cards[].multiverse_id` |
| source | VARCHAR(128) NULL | 翻译来源标记。 | `zhs_card.json: cards[].source` |
| extra | LONGTEXT NULL | 原样保存的补充信息。 | `zhs_card.json: cards[].extra` |

### `flavor_translation`

列数：15。

作用：保存风味文字的中文翻译、来源和审核阶段。

关键约束：表结构和字段语义保持既有设计，`flavor_id` 为主键；仅由 `zhs_flavor.json` 提供。`*_stage` 表示翻译处理阶段，不能替代文本是否为空的判断。与卡牌的关联在导入时通过新的 AllPrintings 卡面 UUID 写入 `card_flavor_translation_mapping`。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| flavor_id | CHAR(36) | 风味翻译主键。 | `zhs_flavor.json: flavors[].id` |
| name | VARCHAR(512) NULL | 原名称。 | `flavors[].name` |
| flavor_name | VARCHAR(512) NULL | 原风味名称。 | `flavors[].flavor_name` |
| flavor_text | LONGTEXT NULL | 原风味文字。 | `flavors[].flavor_text` |
| set | VARCHAR(16) NULL | 系列代码。 | `flavors[].set` |
| collector_number | VARCHAR(64) NULL | 收藏家编号。 | `flavors[].collector_number` |
| released_at | DATE NULL | 发行日。 | `flavors[].released_at` |
| translated_flavor_name | VARCHAR(512) NULL | 中文风味名称。 | `flavors[].translated_flavor_name` |
| translated_flavor_text | LONGTEXT NULL | 中文风味文字。 | `flavors[].translated_flavor_text` |
| flavor_updated_at | DATE NULL | 翻译更新时间。 | `flavors[].flavor_updated_at` |
| extra | LONGTEXT NULL | 原样保存的补充信息。 | `flavors[].extra` |
| name_source | VARCHAR(128) NULL | 名称翻译来源。 | `flavors[].name_source` |
| name_stage | INT NULL | 名称翻译阶段。 | `flavors[].name_stage` |
| text_source | VARCHAR(128) NULL | 正文翻译来源。 | `flavors[].text_source` |
| text_stage | INT NULL | 正文翻译阶段。 | `flavors[].text_stage` |

### `oracle_translation`

列数：18。

作用：保存按 Oracle 卡面归并的中文名称、类别行和规则文字翻译。

关键约束：表结构和字段语义保持既有设计，`face_oracle_id` 为主键；`oracle_id` 可将多卡面关联到同一 Oracle 对象；翻译版本冲突由导入过程拒绝，不静默覆盖。与卡牌的关联在导入时通过新的 AllPrintings 卡面 UUID 写入 `card_oracle_translation_mapping`。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| face_oracle_id | CHAR(36) | Oracle 卡面翻译主键。 | `zhs_oracle.json: oracles[].face_oracle_id` |
| oracle_id | CHAR(36) NULL | Oracle 对象 ID。 | `oracles[].oracle_id` |
| name | VARCHAR(512) NULL | 原名称。 | `oracles[].name` |
| set | VARCHAR(16) NULL | 来源系列。 | `oracles[].set` |
| collector_number | VARCHAR(64) NULL | 收藏家编号。 | `oracles[].collector_number` |
| released_at | DATE NULL | 发行日。 | `oracles[].released_at` |
| type_line | VARCHAR(512) NULL | 原类别行。 | `oracles[].type_line` |
| oracle_text | LONGTEXT NULL | 原规则文字。 | `oracles[].oracle_text` |
| translated_name | VARCHAR(512) NULL | 中文名称。 | `oracles[].translated_name` |
| name_stage | INT NULL | 名称翻译阶段。 | `oracles[].name_stage` |
| name_source | VARCHAR(128) NULL | 名称翻译来源。 | `oracles[].name_source` |
| translated_type | VARCHAR(512) NULL | 中文类别行。 | `oracles[].translated_type` |
| type_stage | INT NULL | 类别行翻译阶段。 | `oracles[].type_stage` |
| translated_text | LONGTEXT NULL | 中文规则文字。 | `oracles[].translated_text` |
| text_stage | INT NULL | 正文翻译阶段。 | `oracles[].text_stage` |
| text_source | VARCHAR(128) NULL | 正文翻译来源。 | `oracles[].text_source` |
| extra | LONGTEXT NULL | 原样保存的补充信息。 | `oracles[].extra` |
| former_names | LONGTEXT NULL | 压缩 JSON 历史名称列表。 | `oracles[].former_names` |

### `set_translation`

列数：5。

作用：保存系列中文名称。

关键约束：保持既有翻译表设计，`set_id` 为主键，`code` 保留为系列代码。导入时按 `code` 与新的 `mtgnissa_set.code` 对应，不改变翻译表的主键；Token 系列可按其 `parent_code` 复用普通系列翻译，不强制生成独立翻译行。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| set_id | CHAR(36) | 既有系列翻译记录主键。 | `zhs_set.json: sets[].set_id` |
| code | VARCHAR(32) | 与新 `mtgnissa_set.code` 匹配的系列代码。 | `zhs_set.json: sets[].code` |
| name | VARCHAR(512) | 中文系列名称。 | `sets[].name` |
| source | VARCHAR(128) NULL | 翻译来源。 | `sets[].source` |
| stage | INT NULL | 翻译阶段。 | `sets[].stage` |

### `type_translation`

列数：6。

作用：保存英文类型词到中文类型词的既有翻译记录。

关键约束：保持既有翻译表设计，以 `(type_name, type_type)` 为联合主键；同一英文词可按词汇类别保留不同翻译。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| type_name | VARCHAR(255) | 英文类型词。 | `zhs_type.json: types[].type_name` |
| type_type | VARCHAR(64) | 词汇类别。 | `types[].type_type` |
| translation | VARCHAR(512) NULL | 中文翻译。 | `types[].translation` |
| stage | INT NULL | 翻译阶段。 | `types[].stage` |
| created_at | DATETIME(6) NULL | 翻译记录创建时间。 | `types[].created_at` |
| is_funny | BOOLEAN NULL | 是否为趣味牌相关词汇。 | `types[].is_funny` |

### `ruling_translation`

列数：8。

作用：保存英文裁定的中文翻译。

关键约束：保持既有翻译表设计，`ruling_key` 为主键；`oracle_ruling` 通过同名 `ruling_key` 关联该表。英文裁定改从 AllPrintings 读取不改变翻译表字段；已有翻译记录优先，不被英文导入覆盖。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| ruling_key | CHAR(64) | 规范化英文裁定正文哈希，主键。 | 由 `zhs_ruling.json: rulings[].comment` 计算 |
| ruling_id | CHAR(36) NULL | 原裁定记录 ID，仅供追溯。 | `zhs_ruling.json: rulings[].ruling` |
| comment | LONGTEXT | 英文裁定正文。 | `zhs_ruling.json: rulings[].comment`；缺失时由 AllPrintings 裁定补充 |
| translation | LONGTEXT NULL | 中文裁定翻译。 | `zhs_ruling.json: rulings[].translation` |
| translation_source | VARCHAR(128) NULL | 翻译来源。 | `zhs_ruling.json: rulings[].source` |
| translation_stage | INT NULL | 翻译阶段。 | `zhs_ruling.json: rulings[].stage` |
| last_published_at | DATE NULL | 最近发布日期。 | `zhs_ruling.json: rulings[].last_published_at`；可由 AllPrintings 裁定日期更新 |
| extra | LONGTEXT NULL | 压缩后的补充 JSON。 | `zhs_ruling.json: rulings[].extra` |

## 4. 映射表

映射表只表达两个既有对象之间的关联，统一命名为 `<对象一>_<对象二>_mapping`；联合主键由两端关联键构成，不承载被映射对象自身的详情字段。

### `card_flavor_translation_mapping`

列数：2。

作用：关联印刷卡面与风味翻译。

关键约束：`(card_uuid, flavor_id)` 为联合主键；两列分别外键关联新的 `mtgnissa_card` 与既有 `flavor_translation`。导入时使用 AllPrintings 的 `scryfall_id` 完成旧卡 UUID 到新卡 UUID 的重映射后，再写入 `card_uuid`。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | 由 `zhs_flavor.json` 与 `AllPrintings.json` 匹配 |
| flavor_id | CHAR(36) | 风味翻译 ID。 | `zhs_flavor.json: flavors[].id` |

### `card_oracle_translation_mapping`

列数：2。

作用：关联印刷卡面与 Oracle 翻译。

关键约束：`(card_uuid, face_oracle_id)` 为联合主键；两列分别外键关联新的 `mtgnissa_card` 与既有 `oracle_translation`。导入时使用 AllPrintings 的 `scryfall_id` 完成旧卡 UUID 到新卡 UUID 的重映射后，再写入 `card_uuid`。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | 由 `zhs_oracle.json` 与 `AllPrintings.json` 匹配 |
| face_oracle_id | CHAR(36) | Oracle 卡面翻译 ID。 | `zhs_oracle.json: oracles[].face_oracle_id` |

### `oracle_tag_mapping`

列数：3。

作用：关联 Oracle 对象与标签，并保存上游原始权重。

关键约束：`oracle_id` 与 `tag_id` 是两个被映射对象，表名遵循 `<源对象>_<目标对象>_mapping`。`(oracle_id, tag_id)` 为联合主键；`tag_id` 关联 `oracle_tag.tag_id`，`weight` 以原始字符串保存，不限制为数值或正数。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| oracle_id | CHAR(36) | Oracle 对象 ID。 | `oracle-tags.jsonl: taggings[].oracle_id` |
| tag_id | CHAR(36) | 标签 UUID。 | 外层 `oracle-tags.jsonl: id` |
| weight | VARCHAR(16) | 标签关联强度原值。 | `oracle-tags.jsonl: taggings[].weight` |

## 5. Oracle 规则数据表

### `oracle_ruling`

列数：4。

作用：保存 Oracle 级别的官方英文裁定。

关键约束：只读取 `AllPrintings.json: sets[].cards[].rulings[]`；`rulings` 缺失、为 `null` 或空数组时不插入。以 `(oracle_id, ruling_key, published_at)` 为主键，其中 `ruling_key` 为规范化裁定文本的哈希；不保存 `source` 字段。相同 Oracle、相同规范化文本、相同日期只保留一条。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| oracle_id | CHAR(36) | Oracle 对象 ID。 | 外层 `cards[].identifiers.scryfallOracleId` |
| ruling_key | CHAR(64) | 规范化裁定文本的哈希。 | 由 `cards[].rulings[].text` 规范化并计算 |
| published_at | DATE | 裁定日期。 | `cards[].rulings[].date` |
| text | LONGTEXT | 英文裁定正文。 | `cards[].rulings[].text` |

### `oracle_legality`

列数：23。

作用：按 `oracle_id` 聚合普通卡牌的赛制支持状态，供按规则对象快速筛选。

关键约束：`oracle_id` 为主键；每一赛制列存上游原始状态（例如 `Legal`、`Banned`、`Restricted`、`Not Legal`）；`NULL` 仅表示上游没有该赛制键，不表示 `Not Legal`。同一 `oracle_id` 的不同印刷在同一赛制列提供多个状态时，按 `Legal > Restricted > Banned > Not Legal > NULL` 的优先级聚合，保留优先级最高的状态，不再因状态不同而报错；缺少赛制键的印刷不参与覆盖已有非空状态。Token 不写入本表。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| oracle_id | CHAR(36) | Oracle 对象主键。 | `cards[].identifiers.scryfallOracleId` |
| alchemy_status | VARCHAR(32) NULL | Alchemy 状态。 | `cards[].legalities.alchemy` |
| brawl_status | VARCHAR(32) NULL | Brawl 状态。 | `cards[].legalities.brawl` |
| commander_status | VARCHAR(32) NULL | Commander 状态。 | `cards[].legalities.commander` |
| duel_status | VARCHAR(32) NULL | Duel 状态。 | `cards[].legalities.duel` |
| future_status | VARCHAR(32) NULL | Future 状态。 | `cards[].legalities.future` |
| gladiator_status | VARCHAR(32) NULL | Gladiator 状态。 | `cards[].legalities.gladiator` |
| historic_status | VARCHAR(32) NULL | Historic 状态。 | `cards[].legalities.historic` |
| competitivebrawl_status | VARCHAR(32) NULL | Competitive Brawl 状态。 | `cards[].legalities.competitivebrawl` |
| legacy_status | VARCHAR(32) NULL | Legacy 状态。 | `cards[].legalities.legacy` |
| modern_status | VARCHAR(32) NULL | Modern 状态。 | `cards[].legalities.modern` |
| oathbreaker_status | VARCHAR(32) NULL | Oathbreaker 状态。 | `cards[].legalities.oathbreaker` |
| oldschool_status | VARCHAR(32) NULL | Old School 状态。 | `cards[].legalities.oldschool` |
| pauper_status | VARCHAR(32) NULL | Pauper 状态。 | `cards[].legalities.pauper` |
| paupercommander_status | VARCHAR(32) NULL | Pauper Commander 状态。 | `cards[].legalities.paupercommander` |
| penny_status | VARCHAR(32) NULL | Penny 状态。 | `cards[].legalities.penny` |
| pioneer_status | VARCHAR(32) NULL | Pioneer 状态。 | `cards[].legalities.pioneer` |
| predh_status | VARCHAR(32) NULL | PreDH 状态。 | `cards[].legalities.predh` |
| premodern_status | VARCHAR(32) NULL | Premodern 状态。 | `cards[].legalities.premodern` |
| standard_status | VARCHAR(32) NULL | Standard 状态。 | `cards[].legalities.standard` |
| standardbrawl_status | VARCHAR(32) NULL | Standard Brawl 状态。 | `cards[].legalities.standardbrawl` |
| timeless_status | VARCHAR(32) NULL | Timeless 状态。 | `cards[].legalities.timeless` |
| vintage_status | VARCHAR(32) NULL | Vintage 状态。 | `cards[].legalities.vintage` |

### `oracle_tag`

列数：4。

作用：保存 `oracle-tags.jsonl` 中的 Oracle 标签定义。

关键约束：保持既有表设计，`tag_id` 为来源标签 UUID 主键；标签只接受 `object=tag` 且 `type=oracle` 的记录。`aliases` 按源顺序以逗号连接保存，不另建别名表。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| tag_id | CHAR(36) | 标签 UUID 主键。 | `oracle-tags.jsonl: id` |
| label | VARCHAR(255) | 标签显示及检索名称。 | `oracle-tags.jsonl: label` |
| description | LONGTEXT NULL | 标签说明。 | `oracle-tags.jsonl: description` |
| aliases | VARCHAR(1024) NULL | 按源顺序拼接的别名列表。 | `oracle-tags.jsonl: aliases` |

### `oracle_tag_relation`

列数：2。

作用：保存 Oracle 标签定义的层级关系。

关键约束：保持既有表设计，`(parent_tag_id, child_tag_id)` 为联合主键；两列均关联 `oracle_tag.tag_id`。`parent_ids` 和其他记录的 `child_ids` 表达的重复边在整个文件范围去重；禁止自环。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| parent_tag_id | CHAR(36) | 父标签 UUID。 | `oracle-tags.jsonl: parent_ids` 或其他记录的 `child_ids` |
| child_tag_id | CHAR(36) | 子标签 UUID。 | `oracle-tags.jsonl: child_ids` 或其他记录的 `parent_ids` |

## 6. Scryfall 卡牌字段展开表

### `scryfall_card_keyword`

列数：2。

作用：将关键字能力展开为可索引行。

关键约束：`(card_uuid, keyword)` 为联合主键；关键字去除首尾空白并按项目规范去重。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | `AllPrintings.json: cards[].uuid` |
| keyword | VARCHAR(128) | 关键字。 | `cards[].keywords` |

### `scryfall_card_type`

列数：3。

作用：将类别行解析为可筛选的类型记录。

关键约束：`(card_uuid, type_group, type_name)` 为联合主键；`type_group` 限定为 `supertype`、`type`、`subtype`。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | `cards[].uuid` |
| type_group | VARCHAR(16) | 类型层级。 | 由 `cards[].supertypes/types/subtypes` 判定 |
| type_name | VARCHAR(128) | 类型名称。 | `cards[].supertypes`、`types`、`subtypes` |

### `scryfall_card_frame_effect`

列数：2。

作用：将卡框效果展开为可筛选行。

关键约束：`(card_uuid, frame_effect)` 为联合主键。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | `cards[].uuid` |
| frame_effect | VARCHAR(64) | 卡框效果代码。 | `cards[].frameEffects` |

### `scryfall_card_promo_type`

列数：2。

作用：将赠卡类型展开为可筛选行。

关键约束：`(card_uuid, promo_type)` 为联合主键。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| card_uuid | CHAR(36) | 印刷卡面 ID。 | `cards[].uuid` |
| promo_type | VARCHAR(64) | 赠卡类型代码。 | `cards[].promoTypes` |

## 7. Scryfall 项目内置枚举表

以下表是项目使用的稳定枚举；可由项目随版本维护，或由指定稳定上游数据源同步。其代码用于解释前述字段中的代码或位掩码。

### `scryfall_color`

列数：5。

作用：定义颜色代码与颜色位值。

关键约束：`code` 为主键，`bit_value` 唯一且为单一二进制位。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(8) | 颜色代码。 | 项目内置字典 |
| bit_value | BIGINT UNSIGNED | 位掩码位值。 | 项目内置字典 |
| name | VARCHAR(64) | 英文名称。 | 项目内置字典 |
| description | VARCHAR(255) NULL | 说明。 | 项目内置字典 |
| sort_order | SMALLINT UNSIGNED | 排序值。 | 项目内置字典 |

### `scryfall_language`

列数：4。

作用：定义语言代码与显示名称。

关键约束：`code` 为主键；与 `mtgnissa_card_printing.language_code` 及系列语言列表使用同一代码体系。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(8) | 语言代码。 | 项目内置字典 |
| name | VARCHAR(64) | 英文语言名。 | 项目内置字典 |
| display_name | VARCHAR(64) | 展示名称。 | 项目内置字典 |
| sort_order | SMALLINT UNSIGNED | 排序值。 | 项目内置字典 |

### `scryfall_layout`

列数：4。

作用：定义卡牌版式。

关键约束：`code` 为主键；`face_category` 用于多面卡面处理，枚举值由项目约束。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(32) | 版式代码。 | 项目内置字典 |
| name | VARCHAR(64) | 版式名称。 | 项目内置字典 |
| description | VARCHAR(255) NULL | 版式说明。 | 项目内置字典 |
| face_category | VARCHAR(32) | 卡面类别。 | 项目内置字典 |

### `scryfall_frame`

列数：3。

作用：定义卡框版本。

关键约束：`code` 为主键。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(16) | 卡框代码。 | 项目内置字典 |
| name | VARCHAR(64) | 卡框名称。 | 项目内置字典 |
| description | VARCHAR(255) NULL | 卡框说明。 | 项目内置字典 |

### `scryfall_frame_effect`

列数：2。

作用：定义卡框效果代码。

关键约束：`code` 为主键。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(64) | 卡框效果代码。 | 项目内置字典 |
| description | VARCHAR(255) NULL | 效果说明。 | 项目内置字典 |

### `scryfall_finish`

列数：3。

作用：定义印刷闪处理及其位值。

关键约束：`code` 为主键，`bit_value` 唯一且为单一二进制位。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(32) | 闪处理代码。 | 项目内置字典 |
| bit_value | BIGINT UNSIGNED | 位掩码位值。 | 项目内置字典 |
| name | VARCHAR(64) | 显示名称。 | 项目内置字典 |

### `scryfall_game`

列数：3。

作用：定义可用游戏客户端及其位值。

关键约束：`code` 为主键，`bit_value` 唯一且为单一二进制位。

| 列名 | 列类型 | 列说明 | 数据来源 |
| --- | --- | --- | --- |
| code | VARCHAR(32) | 游戏客户端代码。 | 项目内置字典 |
| bit_value | BIGINT UNSIGNED | 位掩码位值。 | 项目内置字典 |
| name | VARCHAR(64) | 显示名称。 | 项目内置字典 |

## 8. 查询路径

- 默认卡牌展示：`mtgnissa_card` 通过其保留的 `scryfall_id` 直接关联翻译和图片业务；不需要经过语言变体表。
- 指定印刷语言时：`mtgnissa_card_printing -> mtgnissa_card`，使用该行的印刷文字与 `scryfall_id`；语言变体没有图片 ID 时回退到基础卡面的 `scryfall_id`。
- 系列语言列表只描述普通系列可用语言；Token 不以系列语言维度区分。
- 赛制查询以 `oracle_legality` 为准，裁定查询以 `oracle_ruling` 为准。
