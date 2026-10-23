# 局部显现 / Local Manifestation — Linux 独立服务端

解压后在此目录运行（无需安装 Go）：

```sh
./server -listen 0.0.0.0:7777 -players 8 -tps 30
```

原版内容已嵌入程序；玩家使用相同版本的 Windows 客户端连接本机的可访问地址及 TCP 7777 端口。默认人数上限8，允许单人开始。公网部署需要自行配置防火墙和网络入口。

玩家数据及检查点写入当前目录下的 `data/`，更新版本前请备份。`BUILDINFO.json` 记录本包的版本与源码提交。依赖许可见 `third-party/`。

可通过进程环境变量 `HSM_ADMIN_TOKEN` 设置管理员凭据，在客户端加入服务器时填写。请在进程或服务配置中设置，避免将实际凭据写进共享脚本或提交到仓库。

Run `./server -listen 0.0.0.0:7777 -players 8 -tps 30` from the extracted directory. No Go installation is required. Clients must use the matching Windows release and a reachable TCP address. Save data is stored in `data/`; back it up before upgrades. Set `HSM_ADMIN_TOKEN` in the server environment to configure administrator authentication.
