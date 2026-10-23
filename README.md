# 局部显现 / Local Manifestation

当代背景的 2D Roguelike 合作探索游戏。支持单人及默认最多 8 人联机，队友可以分房行动，服务端统一裁定。通过装备、饰品、武器技能与不同乘区构筑角色。每次升级六维各增加 1 点，并获得机体成长偏好的额外点数。

A contemporary 2D co-op roguelike, built with Go and Ebitengine. Explore together or in separate rooms, build your loadout, and combine damage multipliers.

项目处于开发阶段，当前版本 **0.8.0**。首版客户端支持 Windows，独立服务端支持 Windows / Linux；内容与数值仍在迭代。

使用已解压的开发包时，直接运行 `client.exe`，详细操作见[玩家说明](docs/player-guide.md)。

下载版本见 [GitHub Releases](https://github.com/Tonywww2/Local-Manifestation/releases)。Windows下载游戏ZIP，完整解压后运行 `client.exe`；Linux下载独立服务端tar.gz。每个版本提供 `SHA256SUMS`，发布流程见[Release说明](docs/releases/README.md)。

当前72件物品、13种敌人、全部战斗模板与功能场景已接入统一的当代工业手绘贴图，并补齐机体与大厅设施。[美术规格、完整提示词和预览](docs/art-v1/README.md)；原图独立归档，不进入运行资源包。

## 构建与运行

从源码构建需要 Go（工具链版本由 `go.mod` 指定，当前为 1.26.8）。在 Windows PowerShell 中运行：

```powershell
.\tools\build.ps1
.\bin\client.exe
```

从主菜单进入个人大厅，走近装置按 **F**，即可开始单人游戏、创建联机大厅或加入服务器。局域网队友使用房主的 IP 与端口连接，默认端口为 **7777**。

默认操作：**WASD** 按屏幕方向移动、左键攻击、**右键武器技能**、**空格**闪避、**1–4** 消耗品、**F** 交互、**M** 地图。鼠标移动模式保留右键移动，改用中键释放武器技能。按住 **Q/E** 旋转，滚轮缩放；大地图左键拖动。语言、按键和 HUD 布局可在设置中调整。设置 → 性能与网络可开启帧数、网络延迟与分阶段性能监控。

节点场地支持一个或多个基础六边形拼合，基础尺寸为 1.0 倍（800×500 包围框）。战斗结束后留在原节点与原位置，跨节点移动只允许在战斗外通过大地图确认。

跨房通过大地图：个人移动可沿完整最短通路直接到达目标，每条连接消耗 1 点个人行动力。商店、通路等不阻挡；战斗及特殊事件默认阻挡，完成后可经过。共享同行投票仅限相邻节点，统一消耗 1 点共享行动力。商店可个人进入。战斗中不能跨节点，完成后原地停留。

```powershell
# 独立服务端
.\bin\server.exe -listen 0.0.0.0:7777 -players 8

# 生成 Windows 开发包
.\tools\package.ps1
```

构建产物在 `bin/`；打包文件为 `LocalManifestation-dev-windows-amd64.zip`。本地缓存、玩家数据和编译产物不纳入 Git。

## 项目结构

| 目录 | 用途 |
| --- | --- |
| `cmd/` | 客户端、服务端、模组工具入口 |
| `internal/` | 客户端、场景模拟、冒险协调与宿主适配 |
| `pkg/` | 共享数值、状态、背包、协议与纯逻辑 |
| `sdk/` | 扩展接口与值契约 |
| `content/` | 原版 JSON / JS 内容与资源 |
| `locales/` | 中文与英文本地化 |
| `tools/` | 构建与打包脚本 |
| `examples/` | 扩展示例 |
| `docs/` | 设计、开发指南与阶段记录 |

## 文档

- [玩家说明](docs/player-guide.md) · [开发指南](docs/development.md)
- [已确认设计](docs/design.md) · [架构说明](docs/architecture.md)
- [当前实装与待测](docs/implementation-audit.md) · [文档索引](docs/README.md)
- [开发规范](AGENTS.md) · [模组指南](docs/mods.md)
