# go-autopcr

公主连结（PCR）自动清日常的 **Go 重构版**——无头游戏客户端 + 单账号自动化运行器，纯 Go、**零 CGO**、跨平台。由原 Python 项目 [cc004/autopcr](https://github.com/cc004/autopcr) 移植而来。

## 特性

- **无头客户端**：登录（AccessKey 直传 / bilibili 账密冷启动）、传输加密（AES-CBC + 兼容 msgpack）、会话维护、响应折叠为玩家状态。
- **母数据管线**：不依赖 UnityPy 的纯 Go 解包（UnityFS + LZ4）+ rainbow 反混淆 + 在线版本管理与免登录刷新。
- **自动化运行器**：按游戏域分包的模块库（收取 / 查询 / 报告，一律「先查后动」），配置驱动、可单可批、结构化结果。
- **应用服务门面 `app`**：地道 Go（context / 结构体 / error）的产品级操作面，供 CLI、未来的 web server、以及移动端 wrapper 共享。
- **零 CGO**：`CGO_ENABLED=0`，SQLite 采用纯 Go 的 `modernc.org/sqlite`，保证跨平台可移植。

## 快速开始

需要 Go 1.25+。

```sh
make build              # 构建 bin/autopcr-cli 与 bin/datagen（CGO_ENABLED=0）
make test vet           # 单元测试 + 静态检查
make check-cgo          # 校验产物未链接 CGO
```

CLI 子命令：

```sh
autopcr-cli version                                     # 版本 / 构建信息
autopcr-cli probe    --uid <U> --access-key <K>         # 验证传输 / 会话连通性
autopcr-cli inspect  --uid <U> --access-key <K>         # 登录并打印玩家档案 + 母数据示例查询
autopcr-cli refresh                                     # 免登录 / 免凭证刷新母数据到最新版本
autopcr-cli run --list                                  # 列出可用模块与预设
autopcr-cli run --uid <U> --access-key <K> [模块名...]  # 运行自动化模块（位置模块名须放在 flag 之后）
```

也支持 bilibili 账密登录：`--username <账号> --password <密码>`（仅官服 bsdk 渠道；账密→access_key 的冷启动由外壳完成）。

## 架构

**functional core / imperative shell**：确定性的核心（给定账号 + 线上状态 + 配置，Run 输出可复现）与有副作用的外壳分离。

| 层 | 路径 | 职责 |
|----|------|------|
| 应用门面 | `app` | 三前端共享的产品操作面（`Session` / `Run` / `RefreshMasterdata`） |
| 运行器 | `internal/automation` | 模块 / 任务 / 结果 / 注册表框架 + `modules/` 按域分包的模块库 |
| 无头客户端 | `internal/client` | 装配枢纽；`gameapi` 能力面、`masterdata` 只读查询面、`gamestate` 状态 |
| 管道（隐藏） | `internal/client/internal` | transport / session / protocol / discovery（Go internal 规则强制隐藏） |
| 平台 | `internal/platform` | 日志 / 配置 / 路径 |
| CLI 外壳 | `cmd/autopcr-cli` | flag 解析、账密冷启动、验证码求解器——消费 `app`，不直接依赖核心内部包 |

核心凭据只吃 `(channel, uid, access_key)`；登录 SDK 与验证码求解器均属外壳、经端口注入接入，核心不携带二者的实现，故依赖极薄（`codec` / `lz4` / `sqlite`）、便于跨平台移植。

## 许可证与署名

本项目采用 **CC BY-NC-SA 4.0**（署名-非商业性使用-相同方式共享），见 [LICENSE](LICENSE)。

go-autopcr 是 [cc004/autopcr](https://github.com/cc004/autopcr)（同为 CC BY-NC-SA 4.0）的 Go 移植作品。使用须保留署名、限非商业用途，衍生作品须以相同方式共享。
