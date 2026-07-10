# 贡献指南

## 开发

需要 Go 1.25+。

```sh
make build      # 构建（CGO_ENABLED=0）
make test vet   # 单元测试 + go vet
make check-cgo  # 校验产物未链接 CGO
gofmt -l .      # 格式检查（应无输出）
```

## 硬性约束

- **零 CGO**：`CGO_ENABLED=0`，不引入任何依赖 CGO 的包（SQLite 用 `modernc.org/sqlite`）。跨平台可移植是第一原则。
- **自动化模块「先查后动」**：模块正常运行绝不触发游戏业务 error code——动作前先查状态（如领礼物先查礼物箱）。不要靠捕获业务错误来控制流程，也不要把业务码宽大吞成 skip。
- **按域分包**：能力面（`gameapi`）、协议（`protocol`）、母数据（`masterdata`）、模块（`automation/modules`）均按游戏功能域拆子包 + 访问器聚合。
- **核心保持确定性**：核心不携带登录 SDK / 验证码求解器 / 有副作用或行为不明的东西——经端口注入。前端（外壳）负责冷启动、目录、验证码等平台事宜。

## 提交

- 提交信息用英文，遵循 `<type>: <describe>`（`feat:` / `fix:` / `docs:` / `refactor:` / `chore:`）。
- 提交前确保 `make test vet` 与 `gofmt` 通过。

## 许可

贡献即表示同意你的贡献以本项目许可证 **CC BY-NC-SA 4.0** 授权。
