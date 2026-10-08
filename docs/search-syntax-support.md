# 搜索语法支持矩阵

## 1. 文档目的

本文依据 [Scryfall Search Reference](https://scryfall.com/docs/syntax) 整理 MTGNissa 首版搜索语法，是《需求文档》中首版需求基线的组成部分。本文当前保留的全部关键字、别名、比较符和值域均为首版必须实现和验收的范围；未列出的 Scryfall 写法不属于首版。官方页面核对日期为 2026-10-08。

支持状态：

- **是**：已进入第一期需求基线。本文当前保留的关键字均为第一期必须支持的范围。

表格按“字段关键字、操作符、可选值”逐行列出；仅完全同义的别名合并在同一行。

## 2. 通用名称查询

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `<name>` | 无字段前缀的卡名模糊查询 | `lightning bolt` | 是 | 可与其他受支持条件组合 |
| `"<name>"` | 包含空格或标点的卡名短语 | `"lightning bolt"` | 是 | 双引号内作为连续短语处理 |
| `name:` | 名称字段查询 | `name:izzet` | 是 |  |

## 3. Colors and Color Identity

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `c:`、`color:` | 查询卡牌颜色集合 | `c:rg` | 是 | 两者为别名 |
| `id:`、`identity:` | 查询颜色标识集合 | `id:esper` | 是 | 两者为别名 |
| `:` | 颜色集合完全相等 | `c:wu` | 是 | 首版语义等同 `=` |
| `=` | 颜色集合完全相等 | `c=wu` | 是 |  |
| `!=` | 颜色集合不相等 | `c!=wu` | 是 |  |
| `>` | 当前集合是查询集合的严格超集 | `c>wu` | 是 |  |
| `>=` | 当前集合是查询集合的非严格超集 | `c>=wu` | 是 |  |
| `<` | 当前集合是查询集合的严格子集 | `id<esper` | 是 |  |
| `<=` | 当前集合是查询集合的非严格子集 | `id<=esper` | 是 |  |
| `W`、`white` | 白色 | `c:w` | 是 | 两者为别名 |
| `U`、`blue` | 蓝色 | `c:u` | 是 | 两者为别名 |
| `B`、`black` | 黑色 | `c:b` | 是 | 两者为别名 |
| `R`、`red` | 红色 | `c:r` | 是 | 两者为别名 |
| `G`、`green` | 绿色 | `c:g` | 是 | 两者为别名 |
| `<WUBRG combination>` | 无重复的任意颜色字母组合 | `c:wub` | 是 | 字母顺序不影响结果 |
| `c`、`colorless` | 无色 | `id:c` | 是 | 两者为别名 |
| `m`、`multicolor` | 多色，即颜色数量至少为二 | `c:m` | 是 | 可用于颜色或颜色标识查询 |
| `azorius` | 白蓝色组 WU | `c:azorius` | 是 | 公会名 |
| `dimir` | 蓝黑色组 UB | `c:dimir` | 是 | 公会名 |
| `rakdos` | 黑红色组 BR | `c:rakdos` | 是 | 公会名 |
| `gruul` | 红绿色组 RG | `c:gruul` | 是 | 公会名 |
| `selesnya` | 绿白色组 GW | `c:selesnya` | 是 | 公会名 |
| `orzhov` | 白黑色组 WB | `c:orzhov` | 是 | 公会名 |
| `izzet` | 蓝红色组 UR | `c:izzet` | 是 | 公会名 |
| `golgari` | 黑绿色组 BG | `c:golgari` | 是 | 公会名 |
| `boros` | 红白色组 RW | `c:boros` | 是 | 公会名 |
| `simic` | 绿蓝色组 GU | `c:simic` | 是 | 公会名 |
| `bant` | 绿白蓝色组 GWU | `id:bant` | 是 | 碎片名 |
| `esper` | 白蓝黑色组 WUB | `id:esper` | 是 | 碎片名 |
| `grixis` | 蓝黑红色组 UBR | `id:grixis` | 是 | 碎片名 |
| `jund` | 黑红绿色组 BRG | `id:jund` | 是 | 碎片名 |
| `naya` | 红绿白色组 RGW | `id:naya` | 是 | 碎片名 |
| `abzan` | 白黑绿色组 WBG | `id:abzan` | 是 | 楔形名 |
| `jeskai` | 蓝红白色组 URW | `id:jeskai` | 是 | 楔形名 |
| `sultai` | 黑绿蓝色组 BGU | `id:sultai` | 是 | 楔形名 |
| `mardu` | 红白黑色组 RWB | `id:mardu` | 是 | 楔形名 |
| `temur` | 绿蓝红色组 GUR | `id:temur` | 是 | 楔形名 |
| `silverquill` | 白黑色组 WB | `c:silverquill` | 是 | 学院名 |
| `prismari` | 蓝红色组 UR | `c:prismari` | 是 | 学院名 |
| `witherbloom` | 黑绿色组 BG | `c:witherbloom` | 是 | 学院名 |
| `lorehold` | 红白色组 RW | `c:lorehold` | 是 | 学院名 |
| `quandrix` | 绿蓝色组 GU | `c:quandrix` | 是 | 学院名 |
| `chaos` | 蓝黑红绿色组 UBRG | `id:chaos` | 是 | 四色昵称 |
| `aggression` | 白黑红绿色组 WBRG | `id:aggression` | 是 | 四色昵称 |
| `altruism` | 白蓝红绿色组 WURG | `id:altruism` | 是 | 四色昵称 |
| `growth` | 白蓝黑绿色组 WUBG | `id:growth` | 是 | 四色昵称 |
| `artifice` | 白蓝黑红色组 WUBR | `id:artifice` | 是 | 四色昵称 |
| `<number>` | 按颜色数量查询 | `c=2` | 是 | 支持颜色或颜色标识数量 |

## 4. Card Types

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `t:`、`type:` | 查询英文 Oracle 超类别、卡牌类别或副类别 | `t:merfolk` | 是 | 两者为别名；未加引号时允许类型词前缀匹配 |
| `"<type phrase>"` | 查询连续类型短语 | `type:"legendary creature"` | 是 | 不查询翻译后类型文字 |

## 5. Card Text

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `keyword:`、`kw:` | 按完整关键字异能名称查询 | `keyword:flying` | 是 | 两者为别名 |

## 6. Mana Costs

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `mv:`、`manavalue:` | 查询法术力值 | `mv:5` | 是 | 两者为别名；外部查询不使用 `cmc` |
| `:` | 法术力值相等 | `mv:5` | 是 | 等同 `=` |
| `=` | 相等比较 | `mv=5` | 是 |  |
| `!=` | 不相等比较 | `mv!=5` | 是 |  |
| `>` | 严格大于 | `mv>5` | 是 |  |
| `>=` | 大于或等于 | `mv>=5` | 是 |  |
| `<` | 严格小于 | `mv<5` | 是 |  |
| `<=` | 小于或等于 | `mv<=5` | 是 |  |
| `<nonnegative number>` | 非负整数或小数法术力值 | `mv:2.5` | 是 | 与 `mv:/manavalue:` 组合 |

## 7. Power, Toughness, and Loyalty

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `power:`、`pow:` | 查询力量 | `pow:8` | 是 | 两者为别名 |
| `toughness:`、`tou:` | 查询防御力 | `tou:2` | 是 | 两者为别名 |
| `pt:`、`powtou:` | 查询同一牌面的力量与防御力之和 | `pt:10` | 是 | 两者为别名 |
| `loyalty:`、`loy:` | 查询初始忠诚值 | `loy:3` | 是 | 两者为别名 |
| `:` | 数值相等 | `pow:8` | 是 | 等同 `=` |
| `=` | 数值相等 | `loy=3` | 是 |  |
| `!=` | 数值不相等 | `tou!=2` | 是 |  |
| `>` | 数值大于 | `pow>tou` | 是 | 右侧可为数字或同牌面字段 |
| `>=` | 数值大于或等于 | `pow>=8` | 是 |  |
| `<` | 数值小于 | `tou<pow` | 是 |  |
| `<=` | 数值小于或等于 | `tou<=2` | 是 |  |
| `<number>` | 十进制数值 | `pow:2.5` | 是 | 非纯数值字段不参加比较 |
| `power`、`pow` | 右操作数使用同牌面力量 | `tou<pow` | 是 | 两者为别名 |
| `toughness`、`tou` | 右操作数使用同牌面防御力 | `pow>tou` | 是 | 两者为别名 |
| `loyalty`、`loy` | 右操作数使用同牌面忠诚值 | `pow>loy` | 是 | 两者为别名 |

## 8. Multi-faced Cards

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `is:split` | 查询分割牌 | `is:split` | 是 | 可由布局实现 |
| `is:flip` | 查询倒转牌 | `is:flip` | 是 | 可由布局实现 |
| `is:transform`、`is:tdfc` | 查询转化式双面牌 | `is:transform` | 是 | 两者为别名 |
| `is:meld` | 查询全部融合相关卡牌 | `is:meld` | 是 |  |
| `is:leveler` | 查询升级牌 | `is:leveler` | 是 |  |

## 9. Spells, Permanents, and Effects

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `is:spell` | 查询可施放为咒语的牌 | `is:spell` | 是 | 可由类型派生 |
| `is:permanent` | 查询永久物牌 | `is:permanent` | 是 | 可由类型派生 |
| `is:historic` | 查询史迹牌 | `is:historic` | 是 | 可由类型派生 |

## 11. Rarity

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `r:`、`rarity:` | 按印刷稀有度查询 | `r:common` | 是 | 两者为别名 |
| `:` | 稀有度相等 | `r:rare` | 是 |  |
| `=` | 稀有度相等 | `r=rare` | 是 |  |
| `!=` | 稀有度不相等 | `r!=rare` | 是 |  |
| `>` | 稀有度高于查询值 | `r>uncommon` | 是 | 按官方稀有度顺序比较 |
| `>=` | 稀有度高于或等于查询值 | `r>=rare` | 是 |  |
| `<` | 稀有度低于查询值 | `r<rare` | 是 |  |
| `<=` | 稀有度低于或等于查询值 | `r<=rare` | 是 |  |
| `common`、`c` | 普通 | `r:common` | 是 | 两者为别名 |
| `uncommon`、`u` | 非普通 | `r:uncommon` | 是 | 两者为别名 |
| `rare`、`r` | 稀有 | `r:rare` | 是 | 两者为别名 |
| `special`、`s` | 特殊 | `r:special` | 是 | 两者为别名 |
| `mythic`、`m` | 秘稀 | `r:mythic` | 是 | 两者为别名 |
| `bonus`、`b` | Bonus | `r:bonus` | 是 | 两者为别名 |

## 12. Sets and Blocks

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `s:`、`e:`、`set:`、`edition:` | 按系列代码或名称查询 | `e:war` | 是 | 四者为别名 |

## 27. Using OR

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `OR`、`or` | 两侧条件任一满足即可 | `t:fish OR t:bird` | 是 | 仅大小写不同 |

## 28. Nesting Conditions

| 具体查询关键字 | 关键字查询说明 | 样例 | 是否支持 | 备注 |
| --- | --- | --- | --- | --- |
| `(` | 开始一个嵌套条件组 | `t:legendary (t:goblin OR t:elf)` | 是 | 必须与 `)` 配对 |
| `)` | 结束一个嵌套条件组 | `t:legendary (t:goblin OR t:elf)` | 是 | 必须与 `(` 配对 |
| `<space>` | 使用空白表示隐式 AND | `c:r t:instant` | 是 | 隐式 AND 的优先级高于 OR |
