# Release 流程

推送 `v版本号` 标签自动触发 `.github/workflows/release.yml`。Windows 与 Linux 分别在原生系统构建，使用 `go.mod` 中指定的工具链；只编译和打包，不自动执行全量测试。标签去掉 `v` 后必须与 `content/base/manifest.json` 的版本完全一致。

成功后发布三个附件：Windows AMD64 完整游戏 ZIP、Linux AMD64 独立服务端 tar.gz、`SHA256SUMS`。Linux 包保留可执行权限，两种包均附依赖许可和 `BUILDINFO.json`。本地数据、缓存、性能日志和生成贴图的原始大图不进入发布包；运行贴图与原生扩展所需SDK源码仍包含在Windows包中。

构建任务只有仓库读取权限，发布任务才有 `contents: write`。工作流使用仓库自带的 `GITHUB_TOKEN`，无需另配个人令牌；官方动作固定到提交SHA。所有构建及哈希校验成功后，先建草稿并上传全部附件，再正式发布，避免出现没有下载包的空Release。

## 发布下一版

1. 更新内容版本、README及必要的协议/兼容说明。
2. 添加 `docs/releases/v版本号.md`，记录当前版本改动、启动方式与实际验证范围；没有此文件时自动生成GitHub发布说明。
3. 编译并按改动进行必要的少量实机检查，然后提交并推送代码。
4. 创建并推送标签，例如：

```sh
git tag -a v0.9.0 -m "Local Manifestation v0.9.0"
git push origin v0.9.0
```

带后缀的标签（如 `v0.9.0-alpha.1`）会标记为GitHub预发布，内容版本也须写完整后缀。正式标签使用普通Release，开发阶段由发布说明标明。

失败时可在Actions重跑失败任务，或者选择Release工作流的手动运行并填写已存在的标签。已公开的版本拒绝覆盖，请修复后发布新版本；未公开的草稿可以重试上传。同一标签的运行串行执行，不取消正在发布的任务。

## 本地打包

```powershell
# Windows：构建并生成有版本号的游戏包
.\tools\release.ps1 -Version 0.8.0 -Target windows

# 原开发包入口继续可用
.\tools\build.ps1
.\tools\package.ps1
```

Linux使用PowerShell 7执行 `./tools/release.ps1 -Version 0.8.0 -Target linux`。Release包生成在 `dist/`，开发包仍在 `bin/`；均被Git忽略。

Windows校验：`Get-FileHash .\LocalManifestation-0.8.0-windows-amd64.zip -Algorithm SHA256`。Linux校验：将附件放在同一目录后运行 `sha256sum --check SHA256SUMS`；只有下载了两个压缩包时该命令才会同时成功校验两项。
