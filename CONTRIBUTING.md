# 贡献指南

## 开发

需要 Go 1.25+。

```sh
make build      # 构建（CGO_ENABLED=0）
make test vet   # 单元测试 + go vet
make lint       # golangci-lint，须 0 issues
make check-cgo  # 校验产物未链接 CGO
gofmt -l .      # 格式检查（应无输出）
```

⚠️ CI（见 `.github/workflows/ci.yml`）执行 fmt / build / vet / test / CGO 校验，**不含 `make lint`**——lint 是本地提交前的纪律，须自行执行。

硬性约束、包落位规则、错误与日志约定、注释与文档写法，见 [docs/conventions.md](docs/conventions.md)。

## 提交

- 提交信息用英文，遵循 `<type>: <describe>`（`feat:` / `fix:` / `docs:` / `refactor:` / `chore:`）。
- 提交前确保 `make test vet lint` 与 `gofmt` 通过。

## 许可

贡献即表示同意贡献内容以本项目许可证 **CC BY-NC-SA 4.0** 授权。
