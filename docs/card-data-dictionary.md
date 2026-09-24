# 卡牌数据字典

## 1. 文档范围

本文描述 `docs/magic-cards-zhs-data` 下七个 NDJSON 文件的字段。

- `scryfall_card.json`：以 Scryfall Card Object 文档为主要依据。
- 其余 `zhs_*.json`：以 MTGCH OpenAPI 的 `components.schemas` 为主要依据。
- 字段清单仅来自每个文件的第一条记录，没有扫描全部数据。
- `类型` 中的 `?` 表示字段可能为 `null`。
- NDJSON 文件每一行都是一个独立 JSON 对象，不是一个完整 JSON 数组。

### 1.1 可信度

| 标记 | 含义 |
| --- | --- |
| 官方 | 字段名称和含义可由对应 API 文档直接确认。 |
| 映射 | 本地字段由官方字段重命名、单值化或展开得到。 |
| Schema | 字段可在 MTGCH `components.schemas` 的相关响应对象中确认。 |
| 推定 | 文档没有直接定义原始导出字段，含义根据字段名、样本值和响应对象推定。 |
| 未确认 | 仅能确认字段和值类型，无法从指定文档确定完整语义或枚举。 |

## 2. 文件关系

目前可以使用或推定的主要关联如下：

| 主文件字段 | 关联文件字段 | 状态 | 用途 |
| --- | --- | --- | --- |
| `scryfall_card.face_oracle_id` | `zhs_oracle.face_oracle_id` | Schema | 关联某一个牌面的 Oracle 中文翻译。 |
| `scryfall_card.flavor_id` | `zhs_flavor.flavor_id` | Schema | 关联特定印刷牌面的背景叙述翻译。 |
| `scryfall_card.set_id` | `zhs_set.set_id` | 推定 | 关联系列中文名称。 |
| `scryfall_card.uuid` | `zhs_card.card_id` | 推定 | 关联特定印刷及语言版本的中文印刷文字。 |
| `scryfall_card.oracle_id` | `zhs_oracle.oracle_id` | 官方/Schema | 将同一 Oracle 身份的不同印刷版本归组。 |
| `scryfall_card.multiverse_id` | `zhs_card.multiverse_id` | 映射 | 辅助关联 Gatherer 中文印刷数据。 |

`zhs_ruling.ruling` 的外部关联规则没有出现在 MTGCH Schemas 中，目前不能据此确定关联键。

## 3. `scryfall_card.json`

该文件是经过扁平化和重命名的 Scryfall Card Object，并非 Scryfall 原始响应。

### 3.1 标识与结构

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `uuid` | UUID | 未确认 | 当前数据集内部的卡牌记录主键；不是 Scryfall 标准 `id` 字段。 |
| `scryfall_id` | UUID | 映射 | Scryfall Card Object 的 `id`，唯一标识一个具体印刷版本。 |
| `face_index` | integer | 映射 | 扁平化后的牌面序号；样本中单面牌为 `-1`，其他取值规则仍需用多面牌样本确认。 |
| `lang` | string | 官方 | 当前印刷版本的语言代码。 |
| `oracle_id` | UUID? | 官方 | Oracle 身份 ID；同一规则身份的重印版本通常共享此值。 |
| `layout` | string | 官方 | 卡牌版面类型，例如 `normal`、`split`、`transform`、`modal_dfc`。 |
| `face_oracle_id` | UUID? | Schema | MTGCH 使用的牌面级 Oracle 关联 ID。 |
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
| `produced_mana` | string[]? | 官方 | 卡牌能够产生的法术力颜色。 |
| `reserved` | boolean | 官方 | 是否属于 Reserved List。 |
| `toughness` | string? | 官方 | 生物防御力，使用字符串以支持 `*` 等非数字值。 |
| `type_line` | string | 官方 | 英文类别栏。 |

### 3.4 印刷与美术属性

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `artist` | string? | 官方 | 插画作者名称。 |
| `artist_ids` | UUID[]? | 官方 | Scryfall 插画作者 ID。 |
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

该文件保存特定印刷/语言版本的简体中文文字。MTGCH Schemas 没有同名原始对象，API 中相关内容会被合并到 `CardViewSchema` 和 `CardFaceSchema`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `card_id` | UUID | 推定 | 中文印刷记录所关联的本地卡牌 ID，预计关联 `scryfall_card.uuid`。不是 MTGCH 收藏功能中的 `card_id` 语义。 |
| `name` | string | 推定 | 当前印刷版本的简体中文牌名。 |
| `face_name` | string? | 推定 | 当前印刷版本的单独牌面中文名称。 |
| `flavor_name` | string? | 推定 | 当前印刷版本的中文替代牌名。 |
| `type_line` | string? | 推定 | 当前印刷版本的中文类别栏。 |
| `text` | string? | 推定 | 当前印刷版本的中文规则叙述。 |
| `flavor_text` | string? | 推定 | 当前印刷版本的中文背景叙述。 |
| `multiverse_id` | integer? | Schema | 中文 Gatherer Multiverse ID；API 输出中另有 `zhs_multiverse_id`。 |
| `source` | string? | 未确认 | 中文印刷数据来源；样本值为 `Chinese Simplified`，枚举未定义。 |
| `extra` | unknown? | 未确认 | 扩展信息；MTGCH Schemas 未定义其结构。 |

## 5. `zhs_flavor.json`

该文件保存特定印刷版本背景叙述及其中文翻译状态。API 最终主要映射到 `CardFaceSchema.flavor_text_zhs_html`、`flavor_name_zhs` 和翻译来源信息。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `flavor_id` | UUID | Schema | 背景叙述记录 ID，对应 `CardFaceSchema.flavor_id`。 |
| `name` | string | 推定 | 英文卡牌名称。 |
| `flavor_name` | string? | 推定 | 英文替代牌名。 |
| `flavor_text` | string? | 推定 | 原始背景叙述，可能不是英文，取决于原始印刷语言。 |
| `set` | string | Schema | 系列代码。 |
| `collector_number` | string | Schema | 收藏编号。 |
| `released_at` | date | Schema | 该印刷版本发行日期。 |
| `translated_flavor_name` | string? | 推定 | 简体中文替代牌名翻译。 |
| `translated_flavor_text` | string? | 推定 | 简体中文背景叙述翻译。 |
| `flavor_updated_at` | datetime? | 未确认 | 背景叙述翻译更新时间。 |
| `extra` | unknown? | 未确认 | 扩展信息，结构未在 Schemas 中定义。 |
| `name_source` | string? | Schema | 名称翻译来源，对应 `TranslationSourceSchema.name_source`。 |
| `name_stage` | integer | 未确认 | 名称翻译流程阶段；数值枚举未在 Schemas 中定义。 |
| `text_source` | string? | Schema | 背景叙述翻译来源；API 组合对象中接近 `flavor_source`。 |
| `text_stage` | integer | 未确认 | 背景叙述翻译流程阶段；数值枚举未定义。 |

## 6. `zhs_oracle.json`

该文件保存牌面级 Oracle 英文内容及简体中文翻译。API 最终主要映射到 `CardFaceSchema` 的 `*_atomic`、`*_zhs` 字段。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `face_oracle_id` | UUID | Schema | MTGCH 牌面级 Oracle ID，对应 `CardFaceSchema.face_oracle_id`。 |
| `oracle_id` | UUID? | Schema | Scryfall Oracle 身份 ID，用于跨印刷版本归组。 |
| `name` | string | 推定 | 英文牌面名称。 |
| `set` | string | Schema | 用于确定当前翻译记录来源的系列代码。 |
| `collector_number` | string | Schema | 用于确定当前翻译记录来源的收藏编号。 |
| `released_at` | date | Schema | 来源印刷版本的发行日期。 |
| `type_line` | string? | 推定 | 英文 Oracle 类别栏。 |
| `oracle_text` | string? | 推定 | 英文 Oracle 规则叙述。 |
| `translated_name` | string? | 映射 | 简体中文牌名；API 中对应 `CardFaceSchema.name_zhs`。 |
| `name_stage` | integer | 未确认 | 牌名翻译流程阶段，数值枚举未定义。 |
| `name_source` | string? | Schema | 牌名翻译来源。 |
| `translated_type` | string? | 映射 | 简体中文类别栏；API 中对应 `CardFaceSchema.type_line_zhs`。 |
| `type_stage` | integer | 未确认 | 类别栏翻译流程阶段，数值枚举未定义。 |
| `translated_text` | string? | 映射 | 简体中文规则叙述；API 中对应 `CardFaceSchema.oracle_text_zhs_html` 的文本来源。 |
| `text_stage` | integer | 未确认 | 规则叙述翻译流程阶段，数值枚举未定义。 |
| `text_source` | string? | Schema | 规则叙述翻译来源。 |
| `former_names` | array | Schema | 曾用中文名称列表。 |
| `extra` | unknown? | 未确认 | 扩展信息，结构未在 Schemas 中定义。 |

## 7. `zhs_ruling.json`

该文件保存英文裁定及简体中文翻译。MTGCH API 对外输出为 `RulingSchema`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `ruling` | UUID | 未确认 | 裁定记录内部 ID；Schemas 未说明它与卡牌或 Oracle ID 的关联方式。 |
| `comment` | string | Schema | 英文裁定内容。 |
| `translation` | string? | Schema | 简体中文裁定翻译。 |
| `source` | string? | Schema | 裁定来源，例如样本中的 `official`；完整枚举未定义。 |
| `stage` | integer | 未确认 | 裁定翻译流程阶段，数值枚举未定义。 |
| `last_published_at` | date? | 映射 | 最近发布日期；API 输出字段名为 `published_at`。 |
| `extra` | unknown? | 未确认 | 扩展信息，结构未在 Schemas 中定义。 |

## 8. `zhs_set.json`

该文件保存系列简体中文名称。MTGCH API 最终将相关信息合并进 `SetSchema`。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `set_id` | UUID | 映射 | 系列 ID，预计对应 `SetSchema.id` 以及 `scryfall_card.set_id`。 |
| `code` | string | Schema | 系列代码。 |
| `name` | string? | 映射 | 系列简体中文名称；API 中对应 `SetSchema.translated_name`，而非英文 `name`。 |
| `source` | string? | 未确认 | 系列名称翻译来源，枚举未定义。 |
| `stage` | integer | 未确认 | 系列名称翻译流程阶段，数值枚举未定义。 |

## 9. `zhs_type.json`

该文件保存卡牌类别词汇翻译。MTGCH Schemas 没有公开与其对应的原始或响应对象。

| 字段 | 类型 | 来源 | 含义 |
| --- | --- | --- | --- |
| `type_name` | string | 推定 | 英文类别词汇。 |
| `type_type` | string | 推定 | 词汇类别，例如样本中的 `Type`；完整枚举未定义。 |
| `translation` | string | 推定 | 简体中文翻译。 |
| `stage` | integer | 未确认 | 翻译流程阶段，数值枚举未定义。 |
| `created_at` | datetime | 推定 | 词汇翻译记录创建时间。 |
| `is_funny` | boolean | 推定 | 是否属于非正规或趣味牌相关词汇。 |

## 10. 对当前业务的关键字段

| 业务需求 | 建议字段 |
| --- | --- |
| 区分具体印刷版本 | `scryfall_id`；数据库内部也可继续使用 `uuid`。 |
| 跨系列合并同一张牌 | `oracle_id`；多面牌显示还需结合 `face_oracle_id` 和 `face_index`。 |
| 系列内定位 | `set_code` + `collector_number`，必要时再加 `lang`。 |
| 区分实体牌语言 | `lang`，不能用中文翻译是否存在来代替。 |
| 普通/闪卡可用工艺 | `finishes`、`foil`、`nonfoil`。 |
| 卡牌中文名称和规则叙述 | 优先 `zhs_oracle`，不存在时回退 `scryfall_card` 英文 Oracle 字段。 |
| 特定印刷中文文字 | `zhs_card`，通过本地卡牌 ID 或 `multiverse_id` 关联。 |
| 背景叙述中文翻译 | `flavor_id` 关联 `zhs_flavor`。 |
| 系列中文名称 | `set_id` 关联 `zhs_set`。 |
| 卡图 | `scryfall_id`、`face_index`、`layout`；由后端集中生成 Scryfall CDN 地址。 |
| 排序 | `set_code`、`collector_number`、`colors`、`color_identity`、`cmc`、`name`。 |

## 11. 尚需确认

在正式设计查询 SQL 前，需要通过针对性样本或上游数据模型继续确认：

1. `uuid` 与 `zhs_card.card_id` 是否始终一一对应。
2. `face_index` 在 transform、modal DFC、split、adventure、meld 等版面中的取值规则。
3. `multiverse_ids` 被转换为单个 `multiverse_id` 时的选择规则。
4. `stage`、`name_stage`、`type_stage`、`text_stage` 的枚举含义。
5. 各文件 `extra` 的实际数据类型和结构。
6. `zhs_ruling.ruling` 的生成规则以及如何关联卡牌。
7. `zhs_flavor.flavor_text` 是否允许任意原始语言，而不只英文。

## 12. 参考文档

- [Scryfall Card Objects](https://scryfall.com/docs/api/cards)
- [Scryfall Card Images](https://scryfall.com/docs/api/images)
- [MTGCH API 文档](https://mtgch.com/api/v1/docs)
- [MTGCH OpenAPI Schema](https://mtgch.com/api/v1/openapi.json)

