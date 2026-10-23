# 模组开发与组合（SDK 7）

分发采用 ZIP 容器，开发工具暂用 `.hsmod` 扩展名；整个文件的 SHA-256 是安装、下载和组合锁的身份。包内可以有标准 JSON、JS、资源及纯 Go 源码。下划线开头的 JSON 字段用于注释。SHA-256 校验字节一致性，不等于作者签名。

## 清单与内容

根目录必须包含 `manifest.json`，最小数据包示例：

```json
{
  "_comment": "Development content",
  "id": "example",
  "version": "1.0.0",
  "sdk": 7,
  "type": "gameplay",
  "definitions": "definitions.json",
  "scripts": ["behavior.js"],
  "dependencies": {"other-package": ">=1.0.0 <2.0.0"}
}
```

字段按实际使用选填，不要为缺失文件保留路径。支持 `definitions`、`adventure`、`scripts`、`assets`、`overrides` 与 `go`。定义 ID 使用包命名空间，替换其他包定义需要在 `overrides` 显式声明被替换的 ID。作者优先使用版本范围；组合锁保存精确版本、依赖顺序及摘要。支持比较运算、`^`、`~`、空格连接条件和 `||`，不实现全部 npm 范围语法。

可只提供新增或覆盖的部分定义。完整原版目录可用 `validate -dir content/base` 校验；部分包须和依赖一起解析、加载，不能直接用完整目录的校验命令。Go 入口的签名为 `func(sdk.Registrar) error`，只依赖公开 `sdk`/`pkg` 值契约，不能导入宿主 `internal` 包。

## 安装与使用

设置 → 模组中填写包路径和 SHA-256 安装，填写选中的包 ID 与组合文件路径后解析。可以保存多套组合，安装多个版本。组合加载失败保留当前目录；请先加载成功，再创建游戏。整局进行期间（包括层间准备）禁止重载。

命令行示例，在项目根目录运行；发布包中改用当前目录的 `modtool.exe`：

```powershell
.\bin\modtool.exe pack -dir examples/native-demo -out native-demo.hsmod
$taskModHash = (Get-Content native-demo.hsmod.sha256).Trim()
.\bin\modtool.exe install -file native-demo.hsmod -sha256 $taskModHash
.\bin\modtool.exe resolve -out data/combinations/native.json native-demo
.\bin\modtool.exe build -source . -lock data/combinations/native.json -out bin/modded
```

服务器下载操作先取得清单，再下载所需包，逐个验证摘要后保存组合。下载 Go 源码不会自动编译执行；需要玩家手动构建并重启。两端必须使用同一玩法组合。材质包可以只在个人客户端使用。

## Go 构建与热重载

Go 包还需有 `go.mod` 和入口声明：

```json
"go": {
  "package": "example.com/hsm-native-demo/v2",
  "shared": "Shared",
  "server": "Server",
  "client": "Client"
}
```

三个入口均可选，但至少一个存在；`shared` 为共有注册，服务端/客户端只运行各自入口。运行实例的可变状态留在各自处理器中，不能放入跨世界共享的全局变量。示例见 `examples/native-demo`，包含共有命中加成与客户端通知。

构建器将模组单独编译为归档，并利用 Go 缓存静态重链接客户端、服务端、工具。宿主源码在隔离构建目录复制后生成注册文件，原项目源文件不被改写。三个程序都成功才发布新的 `generation-*` 目录和 `active.json`。读取 `active.json` 的 `directory`，启动该目录内的程序；`launch.json` 记录对应组合路径。移动整个安装目录后应重新构建。

开发包附 `sdk-source`，在发布包构建时使用 `-source sdk-source`。设置中的手动构建入口会寻找它；没有 Go 时下载并校验锁定的官方工具链。依赖下载需要网络。首版使用 `CGO_ENABLED=0`，只支持纯 Go 依赖。原版含 Go 和仅数据包的组合都可静态构建。

含 Go 源码的包在管理页标记为不能热重载，预编译也不改变这一限制；修改后重新构建、重启。JSON/JS/资源包可在单局外重载。加载、脚本注册或资源解码失败时保留旧内容。可复用编译结果和静态绘制缓存，但重载不是任意时刻改写运行中的冒险。

## 材质包与资源

材质包 `type` 为 `texture`，只能包含资源与 JSON 元数据，不含玩法定义、JS 或 Go。`assets` 从资源键映射包内文件，例如 `"ui:theme": "theme.json"`；例子见 `examples/texture-demo`。个人材质包不改变玩法摘要。

当前支持 PNG/JPEG、字体、JSON 界面主题/布局及 WAV 音效/音乐。常用键包括 `ui:font`、`ui:theme`、`audio:attack`、`music:adventure`、`player:<机体ID>`、`portrait:<机体ID>`、`weapon:<武器ID>`、`enemy:<敌人ID>`、`icon:<内容ID>`、`floor:<节点或房间定义ID>` 与 `floor:default`。单幅图像不超过 4096×4096，总解码预算 256 MiB。分发包上限 256 MiB，单文件 16 MiB；路径穿越、符号链接、同名/大小写冲突条目会拒绝。主题和资源在客户端提交后清除相关显示缓存，玩法仍由服务端裁定。

世界角色贴图应当不包含固定武器；背包立绘可以使用独立的 `portrait:` 资源。手持武器使用透明 PNG，面向画面右方，默认握点为图像横向22%、纵向中心。角色、立绘、武器、敌人和 `prop:` 设施会在加载时裁去透明边界。可选 `art:layout` 资源指向JSON，例如 `{"weapon_grips":{"example:rifle":{"x":0.42,"y":0.63}}}`；握点相对裁切后的可见内容计算，坐标必须有限且位于0–1。该布局按现有资源覆盖规则整份替换，覆盖时保留仍需使用的其他武器握点。缺省手持外观按近战、枪械、法器分类使用缓存纹理；换装、攻击、技能和翻滚均由同一动作渲染器处理，不要求材质包实现网络逻辑。

物品拥有 `icon:<完整物品ID>` 时在背包、装备、商店、掉落和开发目录中显示独立贴图；空装备槽继续使用槽位轮廓。地面可按 `floor:<节点定义ID>`、`floor:<房间模板ID>`、`floor:<节点类型>` 或 `floor:default` 提供，个人大厅使用 `floor:hall`。设施示例包括 `prop:hall-solo`、`prop:shop`、`prop:revival`、`prop:exit` 和 `prop:obstacle`；家具应使用正交直上俯视顶面。首版完整规格、提示词和预览见 `docs/art-v1/README.md`。

事件注册、操作类型与参数的准确定义在 `sdk/content.go`，数值解释在 `pkg/damage`，内容校验在 `internal/content`。需要新正式玩法时先讨论规则，再扩展这些值契约和宿主实现；不使用脚本直接持有可变世界引用。
