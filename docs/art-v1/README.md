# 局部显现：统一美术资源 v1

本轮为当前内容制作了 **115 张独立生成原图**：72 件物品、13 种敌人、16 种场景地面、4 种机体与10个设施。全部使用内置 `image_gen.imagegen`，逐个资产生成，以同一工具短刀为材质和笔触参考。主菜单保留已批准的现代工作室设计，重新格式化为1920×1080。没有把概念截图当作可交互界面使用。

## 风格

简约当代工业与个人工作室。材质为哑光聚合物、涂漆金属、织物、纸张和磨砂玻璃；手绘块面、干净的深色轮廓、少量明暗层级，缩小后先读剪影。以暖白、灰、炭黑及克制的青绿为共同语言，功能性的小面积红、蓝、琥珀、紫色用于区分物件。冷静、诡异、日常、荒谬是背景气质，不新增世界观或玩法规则。

防具采用安全帽、工作服、护具和隔离用品。饰品是日常配件、工业触片、测量附件与小型电子模块。移动道具通过缺口、折页、环与坐标几何区分，贴图不绘制数值或文字。M4与Model 700保留现实枪械的基本轮廓，其余武器使用实用的切割或实验器具语言。

敌人的造型来自已有名称与外形设定：袋装物、负重箱、透镜、封装体、漏液容器及匿名人形。强化搬运者保留基础轮廓，打开部分外壳露出内部结构。地面是低对比、纯平、无网格的材质，由游戏继续绘制六边形边界、格线和障碍。大厅设施采用严格直上俯视顶面，角色和敌人使用可读的正交立体剪影；旋转时地面与设施跟随世界，角色保持直立。

## 运行规格

| 用途 | 分辨率 | 格式与方向 |
| --- | --- | --- |
| 72件物品图标 | 128×128 | RGBA PNG，透明背景，等比缩放、约10%留白 |
| 11把可装备武器的手持图 | 256×128 | RGBA PNG，持握端在左，输出端向+X |
| 4种机体与复用立绘 | 256×256 | RGBA PNG，空手全身，由已有动作渲染器驱动 |
| 普通敌人 | 256×256 | RGBA PNG，透明边界在加载时裁切 |
| 守卫及两种搬运者 | 384×384 | RGBA PNG，保留BOSS结构辨识度 |
| 16种地面 | 512×512 | 不透明PNG，无网格、文字或固化障碍物 |
| 10个场景设施 | 256×256 | RGBA PNG，正交顶面，静态家具可烘入场景缓存 |
| 主菜单 | 1920×1080 | 不透明PNG，保留左侧实时菜单的安静区域 |

游戏使用的资源在 `content/base/assets/art-v1/`，由 `content/base/manifest.json` 注册。基础美术133个键，追加骨骼动画后共151个键、146个独立运行文件。武器图标与手持图由同一原图分别格式化；背包中的长武器沿格子对角线展示，便于辨认，世界中的手持图继续朝+X。立绘和默认地面可复用文件。逐个运行文件的尺寸、SHA-256、透明信息与解码预算见 [catalog.json](catalog.json)，追加动画后解码预算约141.47 MiB，运行资源共约31.92 MiB。

生成原图放在 `docs/art-v1/sources/`，不进入Go嵌入资源包。旧设计原文件保留。运行图只在加载时解码并上传，绘制时复用纹理；物品缩放使用平滑过滤。场景缓存比较房间模板、地面资源、形状和障碍物，避免跨层复用错误外观。

## 提示词与预览

[统一美术总览](overview.png) · [原生大厅](native-hall.png) · [原生背包](native-backpack.png) · [原生战斗绘制](native-world.png)

下列JSON保存实际完整提示词、内容ID、文件、生成来源及原图/成品SHA-256；生成方式均为内置工具，未使用CLI/API替代。

| 内容 | 完整记录 | 预览 |
| --- | --- | --- |
| 33件饰品 | [accessories.json](accessories.json) | [accessories-preview.png](accessories-preview.png) |
| 28件装备及消耗品 | [equipment.json](equipment.json) | [equipment-preview.png](equipment-preview.png) |
| 11件武器 | [weapons.json](weapons.json) | [weapons-preview.png](weapons-preview.png) |
| 13敌人、16地面 | [world.json](world.json) | [world-preview.png](world-preview.png) |
| 4种机体 | [players.json](players.json) | [players-preview.png](players-preview.png) |
| 2个大厅工作台 | [props.json](props.json) | [props-preview.png](props-preview.png) |
| 8个其余设施 | [fixtures.json](fixtures.json) | [fixtures-preview.png](fixtures-preview.png) |

统一提示词骨架为：生产用途、已有物件的具体主体、哑光手绘当代风格、固定视角和轮廓、透明/不透明要求、无文字/边框/背景杂物。图标不包含固定的稀有度边框、堆叠数、耐久或快捷键，这些继续由界面实时显示。

## 格式化与接入

`tools/artpack` 只裁去空透明边界、等比缩放并排版开发预览，不重画生成内容或修改设计。先编译工具，再对原图运行，例如：

```powershell
.\tools\go.ps1 build -o bin/artpack.exe ./tools/artpack
.\bin\artpack.exe -input docs/art-v1/sources/accessories/grip-ring.png -output content/base/assets/art-v1/accessories/grip-ring.png -width 128 -height 128 -padding 0.1
.\bin\artpack.exe -input docs/art-v1/sources/world/floor-hall.png -output content/base/assets/art-v1/world/floor-hall.png -width 512 -height 512 -mode tile
.\bin\artpack.exe -sheet -input content/base/assets/art-v1/accessories -output docs/art-v1/accessories-preview.png
```

源图和输出必须是不同路径，透明物件必须已有真实alpha，工具不会从背景中抠图。武器握点存于 `art-layout.json`，以加载时裁切后的可见边界为0–1坐标；适配真实枪托和不对称工具，避免统一握点造成手持漂移。

资源键沿用 `icon:<物品ID>`、`weapon:<武器ID>`、`player:<机体ID>`、`portrait:<机体ID>`、`enemy:<敌人ID>`，新增设施使用 `prop:<设施ID>`。地面先按节点定义、再按房间模板、节点类型、默认地面选择。未提供资源时继续显示已有备用图形。

## 四方向骨骼追加

使用内置图片生成工具为4种机体、13种敌人制作17张透明部件图集，身体核心、头部/上部特征、左右臂/附肢、左右腿/下部对应6列，正面、背面、右侧、左侧对应4行。运行图1536×1024，每个部件格256×256，共408个部件；实际加载时裁空边并缓存GPU子图。已经生成的原图保留于`sources/animation/`，不会嵌入运行包。

[玩家与前四种敌人的提示词和来源](animation-players-and-enemies.json) · [其余敌人的提示词和来源](animation-enemies.json) · [逐格透明与边界检查](animation-alpha-audit.json) · [动画系统](../animation-system.md)

[最终原生骨骼画面](native-animation.png)。本轮做了正常动作与拾取提示、旋转后的倒地/死亡两处短时原生绘制检查；发现4类非人形骨骼连接偏差后，仅调整骨骼JSON并复查一处画面。逐帧动作连续性、多人输入和长期游戏表现仍待手测。

生成器的格位不总精确，因此使用[格式化工具](../../tools/format-rig-atlas.ps1)按真实透明空白分界裁出部件，等比居中排到标准格内并保留16px透明边界；每项来源记录24组源/目标矩形和SHA-256。该工具只裁切、缩放和排版，不重画部件；重复头部、附肢、背景残留及方向修正均由图片生成工具处理。最终17张图的408格均非空、格边透明，没有相邻部件跨入。

## 本轮检查

Windows客户端、服务端、模组工具及Linux服务端编译通过。做了资源格式/透明/覆盖/哈希与解码预算检查，以及大厅、背包、战斗三种短时原生绘制检查；发现长武器图标偏小，调整斜向展示与格内占比后复查背包。没有执行全量行为测试、竞态或性能回归。

待手测：不同分辨率与HUD布局中的物品识别、计数/耐久/快捷键覆盖；旋转缩放、左右瞄准、换武器、攻击/技能/翻滚；跨层更换模板、材质重载及缺省图形；多人下队友与敌人的遮挡、密集弹幕和加载瞬间的表现。
