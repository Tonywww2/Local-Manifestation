# 局部显现 / Local Manifestation — 开发版本游玩说明 0.7.0

解压整个目录，运行 `client.exe`。主菜单：开始游戏、设置、退出。开始游戏进入可走动的个人大厅，靠近装置按 G 或点击：单人冒险、房间设置、学习局外加成、创建联机大厅、加入服务器、仓库。

创建大厅会启动独立服务端。其他电脑选择加入服务器，输入房主的局域网地址和端口（默认 7777）。使用相同版本和玩法组合；管理员在准备窗口开始本层。默认单人也能开始，上限 8 人。关闭房主客户端或返回大厅会结束它启动的服务器。独立服务端可单独运行。

默认 WASD 移动、左键朝指针攻击、空格闪避，技能 Q/E/R/F，消耗品 1/2/3/4。F2 切换鼠标移动模式，鼠标模式右键按住直接移动、点击寻路，技能 Q/W/E/R。Tab 切换视角，M 打开大地图。设置可调整语言、按键和 HUD 位置，另有完整玩法说明。

个人大厅右上角护甲图标选择初始机体，点击后保存，适用于单人和联机。战士初始持剑，沿用原卫士数值，不限制后续换装。右下背包 / 状态卡片显示当前武器、金钱、等级、经验数值与进度条；悬浮查看未分配点数，Shift 查看武器计算。顶部显示层数与房间/波次，均可在 HUD 设置中调整位置。基础战士贴图为静态单姿态。

地图只查看。走到真实门口跨房：免费通路连续通过；个人付费入口 G 后确认；共享门与下一层出口靠近装置交互投票，留在范围内维持投票，离开撤票。共享行动力为 0 后可以继续用个人行动力探索和救援；要推进需选择强化 BOSS。最后一点共享行动力用于普通 BOSS 不会强化。

战斗外普通房间也能攻击、闪避和释放技能，仍按资源、冷却与控制效果限制。个人大厅可以攻击和闪避；停止操作时闪避次数也会恢复。

普通战斗开战前、波次间可付费加入，战斗时锁门。波次间仍可移动和攻击，中央显示下一波倒计时。全房倒地暂停该房战斗；外部队友进入或支援，清理后才能救援。全队倒地或死亡判负。第四次倒地死亡；死亡复活后计数为 1，之后第三次倒地再次死亡。倒地惩罚和计数分开。

点击右下卡片打开背包 / 状态：左侧显示角色立绘与武器、四防具；左下显示四消耗品和当前四技能；右上为饰品，右侧其余区域为 27 格背包。左下“属性与加点”打开独立窗口，关闭背包仍保留，可分配点数、滚动查看详细属性，并用 Shift 查看计算。特殊模式扩容后，背包和饰品各自用标题旁箭头翻页；小窗口可滚动内容。

左键拿起整堆，再点放下/合并/交换；右键拿半堆，手持时右键放一个。消耗品每堆最多 30，装备不堆叠，空格不会自动补位；关闭或断线时手持物品回收背包。可在战斗中换装。靠近地面掉落拾取，在商店附近买卖/强化。点击队友状态打开交易，双方确认同一报价；物品交换没有房间/距离/战斗限制，不能直接交易货币。已装备及手持物品须先放回背包再报价。

Hover 跟随鼠标只读，Shift 展开计算，没有按钮，不能拖动。普通窗口可同时打开、拖动、关闭和滚动，不暂停游戏。状态左键不会另开页面。个人/共享行动力在小地图左下；闪避储存以血条右侧圆点表示。

通关装备存入局外仓库，主冒险不能从仓库重复带入装备。局外点数、成就和挑战按同次冒险结算一次。学习树、物品数值和随机内容均是开发配置；标准12饰品槽已接入。首批装备和第一层敌人可玩，开发数值尚待粗调；主流程五层，后续层暂时复用首批模板，无限流程待实装。普通与特殊/玩法模组进度隔离。

管理员可导出最近层间检查点，在房间设置指定恢复文件并开启恢复。留空使用上次文件。独立服务端支持 `server.exe -restore FILE`。恢复需要相同版本/模组摘要，旧成员需在本层开始前回来。不保存战斗中途。

F3 切换中英文，F4 帧率上限，F5 垂直同步，F6 性能计数。逻辑默认 30 TPS，可配置 60，快照频率与渲染独立。实际帧率也取决于机器性能。

同电脑两个客户端用不同 profile，例如：

```powershell
.\client.exe -name Alice -profile alice
.\client.exe -name Bob -profile bob
```

Extract the whole archive and run client.exe. Start Game enters your walkable personal hall. Walk to a device and press G. Solo starts immediately; Host opens a multiplayer lobby; Join uses the host's LAN address and port. Use the same game version and gameplay combination. The administrator starts each floor during preparation.

WASD moves, LMB attacks toward the pointer, Space dodges, Q/E/R/F use skills, and 1–4 use consumables. F2 switches mouse mode (RMB move; Q/W/E/R skills). Tab switches camera; M opens the inspect-only map. Walk between rooms. Personal entrances require payment confirmation; shared entrances and exits use nearby voting devices. Leaving their range withdraws a vote.

Clear rooms, build equipment and defeat the floor BOSS. When shared AP reaches zero, personal AP still permits exploration/rescue; progressing requires an enhanced BOSS. Teammates can occupy separate combat rooms. A completely downed room pauses until help arrives; rescue requires clearing combat. A completely downed/dead team loses.

The bottom-right Inventory / Status card shows the equipped weapon, money, level and XP. Open it for a character portrait and equipment on the left, consumables and configured skills below, accessories at the upper right and 27 fixed bag slots in the remaining right area. Attributes / Points opens a separate window that stays open after inventory closes. Scroll detailed stats and hold Shift for calculations. Expanded bags and accessories have separate page arrows.

There are 9 separate weapon/armor/consumable slots and 12 standard accessory slots. Left click picks up or places a stack; right click takes half or places one while holding. Consumables stack up to 30; equipment never stacks. Closing the bag or disconnecting returns held items. Exchange bag items through teammate status, with both players confirming; currencies cannot be directly traded. Hover follows the mouse and Shift expands calculations. Ordinary windows can be dragged and coexist without pausing. Settings include English/Chinese, key bindings, HUD layout and detailed rules.

Protocol 13 / SDK 6 / content format 3 / checkpoint 6. Update client, server and content together; old default-content checkpoints have a different digest and are unsupported. See README.md, approved-systems.md, mods.md and implementation-audit.md. Public hosting and Steam Workshop are outside this batch. This build uses development content; formal art and balance are pending. The project's distribution license has not been selected.
