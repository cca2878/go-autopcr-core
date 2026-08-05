# 编码与文档约定

本文档是 [CONTRIBUTING.md](../CONTRIBUTING.md) 的展开版本，供修改本仓库代码时参照。架构层面的设计理由见 [architecture.md](architecture.md)。

## 硬性约束

- **零 CGO**：`CGO_ENABLED=0`，不引入任何依赖 CGO 的包。SQLite 用 `modernc.org/sqlite`（纯 Go 实现），UnityFS 解包用纯 Go 的 LZ4。跨平台可移植是第一原则——违反这条会让 gomobile 交叉编译等场景直接失败。
- **自动化模块"先查后动"**：模块正常运行绝不触发游戏业务 error code——动作前先查状态（如领取礼物前先查礼物箱）。不要靠捕获业务错误来控制流程，也不要把业务码宽大处理成 Skip。
- **按域分包**：能力面（`gameapi`）、协议（`protocol`）、母数据（`masterdata`）、模块（`automation/modules`）均按游戏功能域拆子包，顶层以访问器聚合。新增域即新建同名子包，四条树尽量保持域名一致（已知例外见 [architecture.md](architecture.md#四条按域分包的平行结构)）。
- **核心保持确定性**：核心不携带登录 SDK、验证码求解器，也不携带任何其它有副作用或行为不明的实现——一律经端口注入，由外壳（imperative shell）负责装配。核心内部涉及时间的判断一律用 `gc.ServerTime()`，不用 `time.Now()`。详见 [architecture.md](architecture.md#确定性契约)。
- **不导出不必要的符号**：`internal/client`、`internal/automation` 一律不对外提升；`app` 是全仓唯一公开包。新增内容时应先自问"外部是否确实需要直接访问这个类型/函数"，答案不确定时先留在 internal。

## 包落位规则

- **外部要用的类型 → 可见层**（`internal/client` 直接子包，如 `credential`、`gamestate`、`masterdata`、`gameapi`）：出现在 `client.GameClient` 公开签名里的类型都属于这一层。
- **纯实现细节 → 隐藏层**（`internal/client/internal`，如 `transport`、`session`、`protocol`、`urlx`、`discovery`）：Go 的 internal 规则强制只有 `internal/client/...` 内部能导入，外层（`internal/automation`、`app`）均无法访问。新增协议/传输相关代码时默认放在这一层，除非有明确理由需要对外可见。
- **共用模型的落位判据**：一个响应/请求模型只要跨 2 个及以上域包出现，就应定义在 `protocol` 根包而不是各域包各写一份——判据是"协议上它是否跨域"，不是"现在有几处在用"。

## 错误归属规则

- 各域自己的错误类型归各域所有：资源包格式不符归 `unityfs`、母数据构建归 `masterdata`、凭据参数归 `credential`/`accesskey`、模块参数校验归 `automation`。
- `internal/client/gameerr` 的边界**有意划窄**：只收"和游戏服务器打交道时出的事"——链路不通、响应解不开、服务端回业务错误、触发风控、会话被丢弃，以及应中止整条流程的致命态。程序侧自身的错误不属于这里，混在一起会使"游戏服务器出了问题"与"我们自己的代码/数据有问题"这两类在诊断时难以区分，而它们的处置方式完全不同。
- 全仓统一的顶层归类是 `internal/errs` 的 `Domain × Kind`（详见 [architecture.md](architecture.md#错误体系domain--kind)），新增错误类型时应实现 `ErrorClass() errs.Class` 让它能被 `errs.Classify` 定位到。
- 门面 `app` 只提升外壳"获取后确实能做出不同动作"的错误类型；只是"类型不同但处置相同"的不提升。

## 日志级别规则

**谁知道后果，谁记 Error。** 一个错误在某一层可能是即将自愈的正常波动，只有确定不会自愈的那一层才该记 Error；否则一次成功的自愈也会在日志里留下一条误导性的错误记录。具体分级见 [architecture.md](architecture.md#日志谁知道后果谁记-error)，改动传输层或会话守卫的日志级别前，应先确认 `transport/logging_test.go` 是否已钉住对应行为。

`internal/automation` 与 `app` 两层有意不持有 logger——`automation` 是确定性核心，进度信息走 `Observer`/`Collector`，不做 IO；`app` 只做装配转发。若确实需要在这两层加日志能力，先确认不会破坏"核心无 IO"这条边界。

## URL 拼接约定

base URL 一律用 `urlx.ParseBase` 解析（保证路径以 `/` 结尾），相对引用（端点路径）不带前导 `/`，统一用 `(*url.URL).ResolveReference` 拼接。这是为了绕开 `ResolveReference` 遵循 RFC 3986 时的一个陷阱：若 base 路径不以 `/` 结尾，解析相对引用会丢弃 base 路径的最后一段。把规整放在解析阶段即可根除这个问题，调用点无需再关心。

## 注释与文档写法

- **仓内文档一律中文**：docstring、行内注释、README、`docs/` 下的说明文档，统一用中文书写。运行期面向用户的错误消息字符串、`go.mod` 等工具生态要求的英文字段除外。
- **写现状，不写变更史**：注释与文档面向"当下是什么、我该注意什么"的读者，不是项目开发备忘。避免"不再从 X 推断""曾用 Y""改后""返工""第一版"这类只有经历过那段开发的人才能解析的历史参照。决策理由该留在 commit message 里，不留在源码注释和 README 里。
- **正式文档用书面语**：README、`docs/` 说明、公开 API 的 doc comment 一律用书面语——完整句式、术语而非俗语，避免"别""就行""干脆""顺带""其实""坑"这类口语化表达渗入。
- **多说现状，少说为什么**：现状（接口、参数、返回、约束、边界行为）必须写足；"为什么"只在删掉它读者就会用错/改错时才保留。判断信号：句中出现"故""因此选择""刻意""便于""更适合""之所以"，且删掉不影响读者理解时，通常可以删。
- **godoc 不解析 Markdown 强调语法**：`**加粗**`、`_斜体_` 这类写进 doc comment 里会原样显示星号/下划线，不会渲染成加粗。需要强调时靠句子结构或引号承担，不要用 Markdown 强调符号。
- **引号两级分工，均用 ASCII 单列宽字符**（不用占两个字符位的全角符号）：
  - `"…"`（双引号）：引述、术语首现、举例引用的具体值
  - `'…'`（单引号）：契约要点、关键定义的强调
  - **不用 ASCII 方括号 `[…]` 做强调或引用**——Go 1.19+ 的 doc comment 把 `[Name]` 当作符号链接语法解析，中文文本里出现方括号会被误判为一批断链。
- **参考项目与 gt 的代称约定**：
  - 提到本项目移植自的原 Python 实现，统一用"参考项目"指代，不写"ref"这个内部代号（`ref/` 作为工作区内真实存在的目录路径除外，字面路径必须保留，不能替换）。
  - 涉及验证码服务时统一用"gt"指代，不写完整的服务商名称——大小写按语法位置自然选取（`Gt`/`GT`/`gt` 均可）。协议决定的字面量（主机名、回调函数名前缀、JSON 字段路径）除外，这些改了会破坏实际功能。
- **Markdown 正文不按宽度硬换行**：软换行交给编辑器处理。语义分隔处（自然段之间、论点转折处）的手动换行是好的、该保留的，判据是"这个换行是为了表达，还是只因为行长了"。代码围栏、标题、表格、引用块各自独立成行，不受此限制。

## 提交纪律

```sh
make build              # go build ./...
make test vet            # 单元测试 + go vet
make lint                 # golangci-lint，须 0 issues
make check-cgo           # 校验产物未链接 CGO
gofmt -l .                # 格式检查，应无输出
```

提交前确保以上全部通过。CI（`.github/workflows/ci.yml`）执行 fmt / build / vet / test / CGO 校验，**不包含 `make lint`**——lint 是本地提交前的纪律，需要自行执行，不要依赖 CI 兜底。

提交信息用英文，遵循 `<type>: <describe>`（`feat:` / `fix:` / `docs:` / `refactor:` / `chore:`），参考近期提交历史保持风格一致。

贡献即表示同意贡献内容以本项目许可证 **CC BY-NC-SA 4.0** 授权。
