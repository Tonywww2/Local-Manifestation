# 内容定义与 SDK

JSON 定义房间、地图生成、物品、状态、成长和奖励，JS 提供注册处理器与事实事件。数值仍为开发配置。当前全部系统见[实装记录](approved-systems.md)，安装、组合与静态构建见[模组指南](mods.md)。本页保留基础接口例子及注明日期的历史性能基线，完整字段以 `pkg/model` 与 `content/base` 为准。

## 文件与版本

原版目录位于 `content/base`，同时嵌入客户端、服务端和工具。外部内容使用相同加载入口，双方通过 `-content content/base` 指定完全相同的目录。

| 文件 | 内容 |
| --- | --- |
| `manifest.json` | 包 ID、版本、SDK、定义文件、冒险文件和 JS 脚本列表 |
| `definitions.json` | 冒险层序列、默认投票比例、武器、机体、敌人和可选状态定义 |
| `adventure.json` | 层、地图模板、节点定义、房间和波次 |
| `behavior.js` | 敌人行为、商店/恢复处理器、事件监听注册 |

当前包版本为 `0.8.0`，冒险格式 **4**、SDK **7**、协议 **19**、检查点 **7**。移除四槽主动技能与手动分配点数，新增武器技能、实例冷却、机体成长偏好、节点阻挡标记与特殊消耗品；场景快照新增玩家动作时间轴。旧开发检查点拒绝加载；客户端、服务端和内容一起更新。

所有文件使用 UTF-8 标准 JSON，下划线开头的字段作为注释，其余未知字段拒绝加载。定义 ID 为 `包ID:名称`，不同种类的定义也不能重复；地图节点实例 ID 独立于定义 ID，同一节点定义可以在不同位置复用。地图中的 `start`、`boss` 字段引用实例 ID，不能使用保留命令名 `enhanced`、`exhaust`。

敌人定义的 `radius` 是以整个身体中心为原点的服务端碰撞圆半径；可选 `visual_radius` 是独立的美术缩放基准（0或省略时兼容使用 `radius`）。两项范围分别为1–80与0–80。原版13类敌人按四方向部件和动作范围校准，修改美术后应重新核对，见[骨骼契约](skeleton-contract.md)。

加载顺序为解析、索引与引用校验、JS 编译、注册与参数校验、目录 SHA-256。错误指出文件、定义或注册 ID、失败字段或引用；失败不产生可用目录。解析、编译与哈希复用已读取的文件字节，避免读取期间编辑文件导致内容与摘要分离。查询返回副本，已发布定义不会随运行实例改变。SHA-256 对完整目录的文件名与原始字节计算，空白和额外文件也影响身份；翻译目录独立于玩法目录。

## 属性来源与纯函数求值

机体保留基础六属性、生命、护甲和速度，可选 `modifiers` 添加属性修正。例如在某个机体定义中加入：

```json
"modifiers": [
  { "id": "base:training-strength", "stat": "strength", "operation": "add", "value": 5 },
  { "id": "base:training-agility", "stat": "agility", "operation": "percent", "group": "training", "value": 0.2 }
]
```

这段是机体对象内的字段示例，数值只用于演示。`stat` 支持六属性及 `max_hp`、`armor`、`speed`、`attack_speed`、`zoom_min`、`zoom_max`；操作支持 `add`、`percent`、`final_add`。百分比用小数表示并必须填写组 ID，普通/最终加算不得填写组。组名任意，同名组加算、不同组相乘，最后执行最终加算。贡献 ID 使用包命名空间，在同一来源内不可重复；非法操作、字段、数值或负的组因子拒绝加载。

缩放上下限 `zoom_min` / `zoom_max` 的开发默认值分别为 0.5 / 2，旧基础快照缺省时沿用默认值。更小的下限允许视野拉远；装备、饰品与状态可用同一套贡献修改它们，计算过程进入角色属性详情。求值下限为 0.01，反向区间拒绝提交。当前缩放由客户端保存于运行状态，不会改变服务端世界坐标或命中判定。

加载时检验并冻结内容；`SourceRef{kind, actor, instance, definition}` 表达独立贡献。`engine:level` 为每级六维各 +1；`engine:chassis_growth` 为机体额外成长。两者保持独立来源，基础值不覆写，属性预览和伤害解释使用同一求值器。

`pkg/stats.Evaluate` 接收基础值和贡献集合，返回派生值和逐步结果，不修改传入数据。中间数值不舍入；属性、护甲和速度最低为 0，生命上限最低为 1，最终生命变化保留比例并精确到两位。移除贡献后重新求值，不反向扣掉之前的结果。

`pkg/damage.Evaluate` 可接收同一属性输入，属性系数步骤嵌入相应来源明细；命名乘区保留 `group` 与全部 `members`，不再仅依赖拼接文本。武器强化作为独立加算步骤记录。显示翻译仍由客户端处理，扩展可使用 `damage.group.<组ID>`、`damage.source.<标签>` 翻译名称。实际伤害和解释共用计算数据；发射的投射物保存独立攻击输入。

检查点 7 重建并精确校验每级基础成长、机体偏好成长、装备及状态贡献，校验实例技能冷却、冒险时间、库存和地图阻挡定义；拒绝缺失、重复及伪造贡献。

SDK 7 的玩家值不再包含 `skills`、`skill_ready` 和主动分配的 `points`。`Weapon.weapon_skill` 引用 `WeaponSkillDef`，冷却在 `ItemInstance.weapon_skill_ready`；跨房、换装和交易保留，通关归档到局外仓库时清零。扩展包 manifest 的 `sdk` 改为 7。

2026-10-07，Windows amd64、Go 1.26.8、i7-13700K 的单次基线：64 项同组属性贡献约 **22.78 µs/op、66,040 B/op、41 allocs/op**（`BenchmarkEvaluateContributions`，包含完整来源明细）。这测量一次属性求值及解释，不代表整帧；后续状态/装备规模变化时复测。运行：

```powershell
.\tools\go.ps1 test -run '^$' -bench BenchmarkEvaluateContributions -benchmem ./pkg/stats
```

## 增加一个房间

可以复制现有 `base:skirmish`，改 ID、尺寸、波次和奖励，然后让一个节点的 `room` 指向新 ID。以下为 `rooms` 数组中的完整新增条目，保留其他条目：

```json
{
  "id": "base:training-room",
  "width": 1000,
  "height": 400,
  "objective": "clear",
  "join": "between_waves",
  "initial_delay": 1,
  "spawn_radius": { "x": 350, "y": 120 },
  "scaling": {
    "hp_factor": 1,
    "hp_per_floor": 0.25,
    "hp_per_extra_player": 0.35,
    "damage_per_floor": 0.2
  },
  "waves": [{
    "enemies": [{ "enemy": "base:stalker", "count": { "base": 2, "per_player": 1, "per_floor": 1 } }],
    "break_seconds": 2,
    "reward": { "xp": { "base": 10 }, "coins": { "base": 5 } }
  }]
}
```

在 `base:combat-a` 节点定义中将 `room` 改为 `base:training-room` 即可使用。增加节点定义时，地图的 `nodes[].definition` 引用该定义。新房间使用已注册的 `base:chase` 敌人行为，无需修改 Go 或 JS。

房间场地和内部网格均为正六边形。`width`/`height` 是尺寸包络，场地半径为 `min(width/2, height/√3)`，中心为 `(width/2, height/2)`；宽高比例不会拉伸形状。碰撞、出生点、导航和绘制共用这个定义。移动仍是连续二维坐标，导航在六边形网格搜索。当前基础包络为 800×500（1.0 倍）；节点可由多个基础六边形拼合，具体配置见下方“节点内的拼合场地”。

| 配置 | 运行规则 |
| --- | --- |
| `run.floors` | 按引用顺序执行，层数量由列表确定 |
| `floors[].personal_ap/shared_ap` | 每层各玩家统一的初始个人值与全队共享池；既有诅咒在此基础上应用 |
| `maps[].nodes` | 六边形轴坐标和显式双向相邻连接；无自环、重复位置或断开的节点，远程连接需要显式通路格 |
| `nodes[].payment/cost` | `none` 必须为 0；`personal`、`shared` 必须为正整数；投票与扣款由协调器管理 |
| `rooms[].objective` | 当前只支持 `clear`，其他目标待对应玩法实现 |
| `rooms[].join` | `between_waves` 或 `never`；初始等待允许加入，全房倒地后的合法救援仍可进入 |
| `count` 与奖励数量 | `base + per_player × 场景参加人数 + per_floor × 当前层号`，层号从 1 开始 |
| 生命缩放 | 敌人基础生命 × `hp_factor` × [1 + `hp_per_floor` × (层号 − 1)] × [1 + `hp_per_extra_player` × (人数 − 1)] |
| 攻击缩放 | 敌人基础攻击 × [1 + `damage_per_floor` × (层号 − 1)] |
| `spawn_radius` | 以场地中心为圆心的椭圆出生范围，敌人按波次组排列后均匀分布；落点按碰撞半径限制到六边形边界 |
| `break_seconds` | 当前波次完成至下一波的等待时间；奖励在该波完成时结算一次 |

数量系数非负，场地尺寸为 200–4096，单波按最多 128 人校验不超过 2048 名敌人。这些是当前执行器的工程限制，不是最终游戏内容规则。人数变化只影响随后生成的波次，不回头增减当前敌人。原版 BOSS 的 `join` 为 `never`；普通与强化版本分别引用房间定义，仍走同一个团队集结入口。

摄像机按尺寸拟合显示范围，鼠标反投影使用同一变换；战略地图按节点坐标拟合位置与点击区域。大量节点的缩放、平移和筛选交互尚未实现。

## 节点处理器

`handler` 引用 `sdk.registerNode` 注册的处理器，`parameters` 按该处理器声明的 schema 校验。`engine:noop` 是宿主提供的空处理器。

```javascript
sdk.registerNode("base:forge", {
  cost_per_level: { type: "integer", minimum: 1, maximum: 10000 },
  damage_per_upgrade: { type: "number", minimum: 0, maximum: 10000 }
}, function (ctx) {
  if (ctx.command !== "upgrade") return [];
  return [
    { kind: "spend_coins", target: ctx.player.id,
      amount: ctx.parameters.cost_per_level * (ctx.player.upgrade + 1) },
    { kind: "upgrade_weapon", target: ctx.player.id, amount: 1 },
    { kind: "weapon_bonus", target: ctx.player.id,
      amount: ctx.parameters.damage_per_upgrade }
  ];
});
```

参数支持 `number`、`integer`、`boolean`、`string`，全部为必填，未知参数拒绝。数值参数需要明确上下界。`ctx` 包含 `player`、`node`、`floor`、`command`、`first_visit`、`parameters`，目前命令为 `enter` 和 `upgrade`。上下文逐次复制，脚本修改其中的对象不会改变世界。

处理器返回操作计划，宿主检查上下文目标、类型、数值、余额和结果上限，再提交整个批次。购买失败不提交任何强化或扣款；入场失败也不扣行动力或记录访问。个人扣款、共享开门、投票和 BOSS 集结不向脚本开放。当前强化按钮仍仅在商店节点使用。

节点操作允许 `spend_coins`、`upgrade_weapon`、`weapon_bonus`、`remove_penalty`、`heal_full`、`grant_xp`、`grant_coins`、`heal`，目标仅为当前操作者。每批最多 128 条，每条金额在 0–1000000；除 `heal`、`weapon_bonus` 外均为整数。`heal_full` 的原版用法是金额 0。移除惩罚不复活死亡玩家。

访问记录按玩家和节点保存。原版恢复节点只在首次访问降低一层惩罚并恢复生命；重复访问及恢复同一层检查点不会重发该收益。

## 事件与生命周期

```javascript
var unsubscribe = sdk.on("base:wave-example", "scene", "wave_completed", function (ctx) {
  // 示例：事实处理只提交所属场景内的操作。
  return [{ kind: "grant_coins", target: ctx.players[0].id, amount: 1 }];
});
// 有需要时调用 unsubscribe()。原版观察器不增加奖励。
```

| 范围 | 事件 | 目标限制 |
| --- | --- | --- |
| `session` | `node_entered` | 已成功进入节点的玩家 |
| `scene` | `hit`、`after_damage`、`wave_completed` | 该场景内的玩家 |

事件返回的操作只允许 `grant_xp`、`grant_coins`、`heal`，整批校验后提交；不能操作另一场景。事件监听按注册顺序调用，当前广播使用监听列表快照，广播中退订在下次生效。每个监听者收到独立上下文，不继承其他监听者对快照的修改。回调不能再次广播事件或在运行中注册新处理器。

`ctx` 带 `scope`、`scene`、`floor`、`time`（战斗逻辑秒数）、`event`、`players`。命中事实区分 `attack_id`、`hit_id`，伤害后事实另带 `segment_id`、`damage_type`、实际生命损失 `amount`；过量伤害只报告实际扣除量。实体 ID 与 `source_definition/target_definition` 分离。没有通用的“造成伤害时”触发入口。

当前支持多伤害段、命中贡献、伤害后事实、受限吸血、武器技能和状态；每次命中只广播一次 `hit`，各伤害段广播各自 `after_damage`。波次基本奖励属于宿主结算；监听失败不撤销已经发生的命中、进房或基本奖励，但本批监听操作不会部分提交。场景回调错误将该场景置为 `faulted` 并显示诊断；节点事实错误返回“已进入，事件失败”的诊断，入场本身保持有效。

JS 注册与每次回调均有 25 ms 执行预算；关闭场景清理监听、虚拟机状态和队列。编译程序可共用，运行时不能跨场景共用。随机种子由冒险种子、层号和节点实例身份派生，避免开房顺序影响随机流。JS 时间使用场景逻辑时间，房间暂停时不会推进战斗时间。

Go 内容对应 `sdk.NodeHandler`、`sdk.EventHandler`、`sdk.Behavior` 与 `sdk.Registrar`；`script.NewRegistered` 为静态适配器提供同一注册阶段，Go 上下文也复制并走宿主操作校验。该入口有契约测试，当前程序尚无 Go 模组自动发现、编译、链接流程。原生 Go 回调属于可信代码，不能由 JS 中断预算限制。

## 校验、打包与发布边界

```powershell
.\bin\modtool.exe validate -dir content/base
.\bin\server.exe -content content/base
.\bin\client.exe -content content/base -name Alice -profile alice

.\bin\modtool.exe pack -dir content/base -out bin/base.zip
$taskExpectedHash = (Get-Content bin/base.zip.sha256).Trim()
.\bin\modtool.exe verify -file bin/base.zip -sha256 $taskExpectedHash
.\bin\modtool.exe validate -file bin/base.zip -sha256 $taskExpectedHash
```

`verify` 检查 ZIP 字节身份与路径；`validate` 进一步运行内容解析、引用及脚本注册校验。读取分发包必须传入期望 SHA-256，不能只相信包内字段。`validate` 输出内容身份及加载阶段耗时，耗时单位为纳秒。

`content.Store` 发布完成的目录。协调器创建时获取目录租约，关闭后释放；大厅与层间准备都继续使用本次已创建冒险的目录。重载必须在关闭旧冒险后进行。失败重载保留旧版本，并发启动冒险或其他发布会拒绝该次提交；含 Go 源码的目录要求重启。该边界已测试，界面、文件监听和组合目录尚未接入，当前使用外部内容时需重启程序。

## 验证与性能基线

2026-10-07，在 Windows amd64、Go 1.26.8、Intel i7-13700K、默认 GOMAXPROCS 24 下执行 50 次固定迭代。目录样本有 2 个机体、2 把武器、2 种敌人、3 层、7 个地图节点、1 个 JS 文件。较大目录仅将房间模板扩展至 100 个；计时使用内存文件系统，不含磁盘冷读、图形、网络或下载。

| 样本 | 每次总耗时 | 分配内存 | 分配次数 |
| --- | ---: | ---: | ---: |
| 加载 3 个房间 | 0.371 ms | 229273 B | 3212 |
| 加载 100 个房间 | 2.746 ms | 2284077 B | 27907 |
| 1 个场景各更新一帧 | 0.029 ms | 25750 B | 364 |
| 8 个场景各更新一帧 | 0.120 ms | 206383 B | 2920 |

加载 100 房间的阶段均值为解析 2.500 ms、校验 0.033 ms、编译 0.073 ms、注册 0.083 ms、哈希 0.062 ms。3 房间的小样本哈希计时显示 0，受计时分辨率影响，不表示没有成本。并行场景样本每场景 2 名玩家、第二层 6 名近战敌人，由各自 goroutine 更新，50 帧内没有发射攻击。结果是工程基线，不是完整战斗或高人数性能承诺。

```powershell
.\tools\go.ps1 test ./internal/content ./internal/scene -run '^$' -bench 'Benchmark(CatalogLoad|ConcurrentSceneStep)$' -benchtime=50x -count=1
```

行为验证包括 JSON 新增房间与地图入口/BOSS 改名的双 TCP 流程、数组重排、非法定义、失败购买和入场、跨场景操作拒绝、波次奖励一次性提交、监听退订与关闭、倒地保留/救援/全队判负、检查点定义和访问恢复、两种视角适配及中英文内容名称。独立设备、公网与 Linux 主机实际运行仍需验证。

## 实际空间补充（协议 7）

`maps[].terrain` 可选，元素为 `q`、`r`、`name`、可选 `name_key`；地形不得与房间或其他地形重叠，不创建可进入空间。`kind: passage` 使用 `payment: personal`、`cost: 1`、`handler: engine:noop` 且不绑定战斗房间，为按次收费且无奖励的独立通路。房间之间必须处于六个轴坐标相邻方向之一且显式双向连接；相邻但没有连接的边界不可穿过。客户端默认预加载当前及第一圈房间的显示资源，未入场房间不提前启动战斗。见[跨房实现](history/doors-stage.md)。

## 装置与生命记录

当前世界交互共用 `pkg/device.Device`；`Entrance` 嵌入基础装置，保存入口类型、付款和费用。节点定义的名称/提示本地化用于装置 Hover；地图跨房与商店事务仍由宿主验证；通路不再生成场内入口装置。大厅与原生 UI 的完整范围见[当前记录](history/device-hall-stage.md)。

`knockdowns` 是累计倒地历史，`knockdown_base` 是死亡判定的基线，`downs` 是可移除的惩罚。移除惩罚不清零历史；第四次倒地判死，普通救援不会复活死亡玩家。死亡复活后通过基线使当前计数为 1，之后第三次倒地再次死亡。检查点 6 保存并验证三者，旧格式明确拒绝加载。

`maps[].generator.passage` 可选，引用通路定义；启用时先生成主要节点及回环连接，再将每条连接分割并插入通路。`count` 为含通路的节点预算，实际数量可少于预算；`extra_edge_chance` 决定额外回环。当前开发配置为 `base:passage` 与 0.8，地图仍由种子、层级及玩家数固定生成。

## 节点内的拼合场地（2026-10-09）

原版基础六边形恢复为 1.0 倍，基础包围框 800×500。`rooms[].width`、`height` 定义一个基础六边形的尺寸，`rooms[].hexes` 可选，使用节点内部的轴坐标列表 `{q, r}`；省略时为单六边形。布局必须包含 `(0,0)`，不能重复，最多 128 格，所有格子需按共边相邻连通。节点内轴坐标与战略地图坐标相互独立，负坐标也可用。

非战斗节点可在 `nodes[].hexes` 中声明相同布局，使用默认基础尺寸；战斗节点统一使用所绑定房间的布局，不能同时声明两套几何。原版已有直线、折角、紧凑三格、多分支四格的战斗场地及双格商店，均为后续可调整的开发配置。

全部小六边形属于同一节点、一次战斗和同一组波次。内部接缝不产生墙或额外行动力消耗；只有外边界及障碍参与碰撞。玩家预测、鼠标寻路、敌人导航、射弹和渲染共用同一份编译几何；凹角不能用直线路径或闪避切过。战斗完成后原位交接为战斗外动作宿主，不重新生成场地。清场不会自动跨节点或打开地图。

## 自动成长、武器技能与连续移动（0.8.0）

机体 `growth_preference` 使用六维同名字段表示每级额外整数点，省略为 0；升级基础六维各 +1 不写入配置。原版开发偏好：战士力量 +2、体质 +1；侦察敏捷 +2、感应 +1；法术智力 +2、魔力 +1；复苏智力 +1、魔力 +2。所有经验授予共用成长事务，不依赖清场。

`definitions.json` 使用 `weapon_skills` 数组。每项有 ID、名称、描述、图标、`cooldown_ms`、可选资源 `cost`、效果列表及可选弹体武器 `weapon`。`clear_projectiles: {"range":180,"arc":110}` 表示以指针方向为中线清除敌对弹幕；不产生伤害或命中事件。原版刀具“断流”冷却 4 秒、消耗 0；手持耦合器“束缚脉冲”冷却 5 秒、消耗 15，均为开发数值。技能失败不扣资源或写入冷却。

节点 `blocking` 可显式覆盖默认值；省略时通路、商店、起点不阻挡，其余阻挡，BOSS 始终不能作为中途节点。完成的阻挡节点可经过。个人移动使用确定性的最短边数路线，终点可为未完成战斗或事件；经过节点不运行进入处理器、不授予收益、不标记玩家首次访问。总费用等于边数，完整校验后原子入场；共享投票仍只走一条相邻连接。商店允许个人进入。

装备槽默认纹理由原有图标系统缓存生成，可通过资源键 `slot:weapon/head/body/legs/feet/consumable/accessory` 覆盖，不影响物品行为。

## 移动道具与被动消耗品（2026-10-10）

`items[].kind` 新增 `movement`、`passive`；实例保存 `durability`。定义耐久为正整数，只有被动允许 `-1` 表示无限；这两类物品不堆叠、不可装备快捷栏。`tier` 同时允许普通消耗品声明 1–3 档。

移动组件 `movement` 使用 `mode: line/ring/anywhere`、可选整数 `range`、`free` 和 `team`。直线基于六边形轴向坐标的六个方向，与屏幕旋转无关；距离包含目的节点。个人道具忽略道路、阻挡与原节点付款类型，每次扣 1 个人行动力，`free` 不扣；`team` 经过投票扣 1 共享行动力。目标战斗入场时间仍由场景所有者裁定。消耗耐久和资源随成功入场提交。

被动组件 `passive` 使用 `trigger: lethal_damage/movement/movement_combat`，可配置 `status` 与 `accessory: true`。触发时先快照背包顺序，再逐件消耗全部匹配实例；泛移动效果也匹配移动到战斗节点。奖励饰品从当层地图引用房间的饰品掉落池去重取权重，失败不提交，满包留在目标节点地面。当前原版普通房间使用相同的饰品池；后续不同层的专属内容需分别配置其房间奖励。

`duration_ms: 0` 只在 `clear_on_leave: true` 时合法，表示房间生命周期，不受普通时间到期影响；使用现有状态上限时间作为内部标记。离场移除，清场后留在原房间则继续生效。致命免疫在伤害结算预计算阶段应用，所有匹配熔断片独立提供施加时最大生命 50% 的盾，保留原生命及护盾，并令该次命中的实际生命损失为零。

地图生成可配置 `generator.boss_approach`，要求通路生成和唯一 BOSS 入口，替换该入口的通路定义。每层 `excluded_random_kinds` 同时过滤随机池和团队局外额外设施。商店仅在 `weapon_upgrade: true` 时提供强化。第一层使用独立地图配置，排除复苏、净化/强化和随机商店，只在 BOSS 前固定补给商店。

名称、图标、价格与获取概率属于开发配置；当前普通战斗最终奖励有一次 30% 的一、二档消耗品抽取，三档新增道具由商店提供。
