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

当前包版本为 `0.6.0`，冒险定义格式为 **3**、SDK 为 **6**、网络协议为 **10**、检查点格式为 **6**。值契约包含属性贡献、独立状态层、冒险时间、库存、成长与结构化伤害明细；旧开发检查点明确拒绝加载，请重新开局。客户端、服务端及内容目录需一起更新。

所有文件使用 UTF-8 标准 JSON，下划线开头的字段作为注释，其余未知字段拒绝加载。定义 ID 为 `包ID:名称`，不同种类的定义也不能重复；地图节点实例 ID 独立于定义 ID，同一节点定义可以在不同位置复用。地图中的 `start`、`boss` 字段引用实例 ID，不能使用保留命令名 `enhanced`、`exhaust`。

加载顺序为解析、索引与引用校验、JS 编译、注册与参数校验、目录 SHA-256。错误指出文件、定义或注册 ID、失败字段或引用；失败不产生可用目录。解析、编译与哈希复用已读取的文件字节，避免读取期间编辑文件导致内容与摘要分离。查询返回副本，已发布定义不会随运行实例改变。SHA-256 对完整目录的文件名与原始字节计算，空白和额外文件也影响身份；翻译目录独立于玩法目录。

## 属性来源与纯函数求值

机体保留基础六属性、生命、护甲和速度，可选 `modifiers` 添加属性修正。例如在某个机体定义中加入：

```json
"modifiers": [
  { "id": "base:training-strength", "stat": "strength", "operation": "add", "value": 5 },
  { "id": "base:training-agility", "stat": "agility", "operation": "percent", "group": "training", "value": 0.2 }
]
```

这段是机体对象内的字段示例，数值只用于演示。`stat` 支持六属性及 `max_hp`、`armor`、`speed`；操作支持 `add`、`percent`、`final_add`。百分比用小数表示并必须填写组 ID，普通/最终加算不得填写组。组名任意，同名组加算、不同组相乘，最后执行最终加算。贡献 ID 使用包命名空间，在同一来源内不可重复；非法操作、字段、数值或负的组因子拒绝加载。

加载时检验并冻结内容；创建玩家时绑定 `SourceRef{kind, actor, instance, definition}`。层级加点通过 `engine:level` 来源贡献表达，基础值不随加点被覆盖。来源身份与组 ID 独立，来自两个物品实例的同名贡献可以分别存在；组是否相加只取决于显式组 ID。库存持有稳定物品实例，装备投影与限时状态已经接入来源系统。

`pkg/stats.Evaluate` 接收基础值和贡献集合，返回派生值和逐步结果，不修改传入数据。中间数值不舍入；属性、护甲和速度最低为 0，生命上限最低为 1，最终生命变化保留比例并精确到两位。移除贡献后重新求值，不反向扣掉之前的结果。

`pkg/damage.Evaluate` 可接收同一属性输入，属性系数步骤嵌入相应来源明细；命名乘区保留 `group` 与全部 `members`，不再仅依赖拼接文本。武器强化作为独立加算步骤记录。显示翻译仍由客户端处理，扩展可使用 `damage.group.<组ID>`、`damage.source.<标签>` 翻译名称。实际伤害和解释共用计算数据；发射的投射物保存独立攻击输入。

检查点 6 校验基础值、机体/成长/装备/限时状态来源、加点预算、冒险时间、倒地记录、库存、地图和派生结果，拒绝错引用、重复身份或被修改的最终属性。Go/JS 回调只获得包含嵌套副本的值上下文。

SDK 6 的玩家值保留 `base_stats`、`stat_contributions`、`statuses`、`knockdowns`，新增库存、技能、资源和复活计数基线。事件上下文区分命中与伤害后事实，客户端快照监听只提交通知。Go 属性值使用 `Attributes.Value` / `Attributes.With`；事务由宿主所有者提交。外部内容需更新 manifest 的 `sdk` 为 6，重新校验并与客户端、服务端一起发布。

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

房间场地和内部网格均为正六边形。`width`/`height` 是尺寸包络，场地半径为 `min(width/2, height/√3)`，中心为 `(width/2, height/2)`；宽高比例不会拉伸形状。碰撞、出生点、导航和绘制共用这个定义。移动仍是连续二维坐标，导航在六边形网格搜索。当前开发包络为 2000×1250，非战斗节点可以通过空间配置调整；相比早期场地扩大 2.5 倍。

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

现阶段只执行单伤害段攻击。`hit` 是已裁定命中的事实，事件操作在本轮战斗更新后提交，不能改变本次伤害公式。正式多伤害段、命中贡献接口、吸血、技能和状态将在对应系统实现时增加。波次基本奖励属于宿主结算；监听失败不撤销已经发生的命中、进房或基本奖励，但本批监听操作不会部分提交。场景回调错误将该场景置为 `faulted` 并显示诊断；节点事实错误返回“已进入，事件失败”的诊断，入场本身保持有效。

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

`maps[].terrain` 可选，元素为 `q`、`r`、`name`、可选 `name_key`；地形不得与房间或其他地形重叠，不创建可进入空间。`kind: passage` 使用 `payment: none`、`cost: 0`、`handler: engine:noop` 且不绑定战斗房间，目前为无奖励免费通路。房间之间必须处于六个轴坐标相邻方向之一且显式双向连接；相邻但没有连接的边界不可穿过。客户端默认预加载当前及第一圈房间的显示资源，未入场房间不提前启动战斗。见[跨房实现](history/doors-stage.md)。

## 装置与生命记录

当前世界交互共用 `pkg/device.Device`；`Entrance` 嵌入基础装置，保存入口类型、付款和费用。节点定义的名称/提示本地化用于装置 Hover；真实门口与商店事务仍由宿主验证。大厅与原生 UI 的完整范围见[当前记录](history/device-hall-stage.md)。

`knockdowns` 是累计倒地历史，`knockdown_base` 是死亡判定的基线，`downs` 是可移除的惩罚。移除惩罚不清零历史；第四次倒地判死，普通救援不会复活死亡玩家。死亡复活后通过基线使当前计数为 1，之后第三次倒地再次死亡。检查点 6 保存并验证三者，旧格式明确拒绝加载。
