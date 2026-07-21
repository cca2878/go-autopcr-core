# go-autopcr-core

> 公主连结（PCR）自动清日常的 **Go 重构版**——无头游戏客户端 + 单账号自动化运行器，纯 Go、**零 CGO**、跨平台。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![CGO](https://img.shields.io/badge/CGO-disabled-success)
![License](https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey)

由原 Python 项目 [cc004/autopcr](https://github.com/cc004/autopcr) 移植而来。对外**唯一公开 Go 包是应用服务
门面 `app`**——CLI、未来的 web server、以及移动端 wrapper（[autopcr-mobile-gocore](https://github.com/cca2878/autopcr-mobile-gocore)）
都消费它，不碰底下的 `internal/`。

## 特性

- **无头客户端**：登录（AccessKey 直传 / bilibili 账密冷启动）、传输加密（AES-CBC + 兼容 msgpack）、会话维护（**严重错误码自动重登**，如被其他客户端顶号）、响应折叠为玩家状态。
- **母数据管线**：不依赖 UnityPy 的纯 Go 解包（UnityFS + LZ4）+ rainbow 反混淆 + 在线版本管理与免登录刷新。
- **自动化运行器**：按游戏域分包的模块库（收取 / 查询 / 报告，一律「先查后动」），配置驱动、可单可批、结构化结果；**配置候选可依赖账号与母数据**（登录后据实解析并校验，而非编译期写死——如彩装选装、黎明界公会）；支持任务级进度推送与边界取消。
- **应用服务门面 `app`**：地道 Go（context / 结构体 / error）的产品级操作面，供三前端共享。
- **零 CGO**：`CGO_ENABLED=0`，SQLite 采用纯 Go 的 `modernc.org/sqlite`，保证跨平台可移植。

## 架构

**functional core / imperative shell**：确定性的核心（给定账号 + 线上状态 + 配置，`Run` 输出可复现，
核心内部用**服务器时间**而非本机 wall clock）与有副作用的外壳（网络 / 登录 / 落盘）分离。

```
app 门面（唯一公开包）
  └─ internal/automation      运行器：模块 / 任务 / 结果 / 注册表 + modules/ 域模块库
       └─ internal/client     无头客户端：gameapi 能力面 · masterdata 只读面 · gamestate 状态
            └─ …/internal     管道：transport / session / protocol / discovery（Go internal 强制隐藏）
```

| 层 | 路径 | 职责 |
|----|------|------|
| 应用门面 | `app` | 三前端共享的产品操作面（`Session` / `Run` / `RefreshMasterdata`） |
| 运行器 | `internal/automation` | 模块 / 任务 / 结果 / 注册表框架 + `modules/` 按域分包的模块库 |
| 无头客户端 | `internal/client` | 装配枢纽；`gameapi` 能力面、`masterdata` 只读查询面、`gamestate` 状态 |
| 管道（隐藏） | `internal/client/internal` | transport / session / protocol / discovery（Go internal 规则强制隐藏） |

核心凭据只吃 `(channel, uid, access_key)`；登录 SDK（[bsdkv3-go](https://github.com/cca2878/bsdkv3-go)）与
验证码求解器（[gtrv-go](https://github.com/cca2878/gtrv-go) 远程 / [gtlv-go](https://github.com/cca2878/gtlv-go)
本地）均属外壳、经端口注入接入，核心不携带二者实现，故依赖极薄（`codec` / `lz4` / `sqlite`）、便于跨平台移植。

**本仓是纯库，无任何可执行产物**：日志 / 配置 / 目录默认值同样是外壳的事（门面的目录一律由调用方
传入，核心不假设工作目录）。开发测试用的 CLI 见 `autopcr-cli`——它作为纯外部消费者存在，因而也是
`app` 门面的活体检验：门面缺了什么，它会第一个编译不过。母数据解包不再需要独立工具，已内化进
`internal/client/masterdata`（登录与 `RefreshMasterdata` 共用 `Manager.EnsureDB`，rainbow 随客户端内嵌）。

## 快速开始

需要 Go 1.25+。

```sh
make build              # go build ./...（CGO_ENABLED=0 下全部包可编译）
make test vet lint      # 单元测试 + 静态检查 + golangci-lint
make check-cgo          # 校验 CGO 确实禁用（读测试二进制的内嵌构建设置）
```

## 作为库消费

```sh
go get github.com/cca2878/go-autopcr-core
```

经 `app` 门面：`app.NewSession(dirs)` → `Login(ctx, channel, uid, accessKey, withMasterdata)` →
`Run(ctx, tasks, obs)`（`obs` 为可选进度观察者）/ `RefreshMasterdata`。参考消费方见 `autopcr-cli` 仓。
移动端经 gomobile 皮 [autopcr-mobile-gocore](https://github.com/cca2878/autopcr-mobile-gocore) 出 Android AAR。

## 许可证与署名

本项目采用 **CC BY-NC-SA 4.0**（署名-非商业性使用-相同方式共享），见 [LICENSE](LICENSE)。

go-autopcr-core 是 [cc004/autopcr](https://github.com/cc004/autopcr)（同为 CC BY-NC-SA 4.0）的 Go 移植作品。
使用须保留署名、限非商业用途，衍生作品须以相同方式共享。
