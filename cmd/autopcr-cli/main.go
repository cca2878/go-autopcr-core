// Command autopcr-cli 是 go-autopcr 的测试用命令行入口。
//
// 它是 go-autopcr 的【第一个 imperative shell】：只做 flag 解析、账密冷启动(bsdk)、验证码
// 求解器构造(remote)、目录/输出等平台相关事宜，把产品操作全部委托给 app 门面——故本文件
// 不直接 import internal/client 或 internal/automation，只经 app 与自有外壳组件工作。
//
// 已实现子命令：
//   - version       显示版本与构建信息
//   - probe   (M1)  用 AccessKey 直连游戏服，验证传输/会话连通性
//   - inspect (M2)  登录并打印无头客户端读取到的玩家档案
//   - refresh (M3)  免登录/免凭证刷新母数据到最新版本（source_ini+maintenance 自取版本）
//   - run     (M3)  登录后运行自动化模块（统一单/批），输出每模块结果
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cca2878/go-autopcr-core/app"
	"github.com/cca2878/go-autopcr-core/cmd/autopcr-cli/internal/bsdklogin"
	"github.com/cca2878/go-autopcr-core/cmd/autopcr-cli/internal/remote"
	"github.com/cca2878/go-autopcr-core/internal/buildinfo"
	"github.com/cca2878/go-autopcr-core/internal/platform/config"
	"github.com/cca2878/go-autopcr-core/internal/platform/logging"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run 执行子命令派发，返回进程退出码（便于测试）。
func run(args []string) int {
	// 平台层引导：加载配置 + 初始化全局日志。后续里程碑的命令共用此范式。
	cfg := config.Load()
	logger := logging.Setup(cfg.Debug)
	logger.Debug("autopcr-cli invoked", "args", args, "cache", cfg.Paths.Cache, "data", cfg.Paths.Data)

	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "version", "-v", "--version":
		fmt.Println(buildinfo.Get().String())
		return 0
	case "probe":
		return loginCommand(cfg, "probe", args[1:], false)
	case "inspect":
		return loginCommand(cfg, "inspect", args[1:], true)
	case "refresh":
		return refreshCommand(cfg, args[1:])
	case "run":
		return runCommand(cfg, args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

// credFlags 汇集各命令共用的凭据/网络 flag。
type credFlags struct {
	uid       string
	accessKey string
	username  string
	password  string
	channel   string
	proxy     string
	insecure  bool
}

// registerCredFlags 在 fs 上登记凭据/网络 flag 并返回接收值的结构体。
func registerCredFlags(fs *flag.FlagSet) *credFlags {
	cf := &credFlags{}
	fs.StringVar(&cf.accessKey, "access-key", "", "游戏 access_key（与 --uid 搭配直传）")
	fs.StringVar(&cf.uid, "uid", "", "游戏 uid（与 --access-key 搭配直传）")
	fs.StringVar(&cf.username, "username", "", "bilibili 账号（与 --password 搭配走账密登录）")
	fs.StringVar(&cf.password, "password", "", "bilibili 密码（与 --username 搭配走账密登录）")
	fs.StringVar(&cf.channel, "channel", app.ChannelBSDK, "渠道：bsdk（官服）| qsdk（渠道服）")
	fs.StringVar(&cf.proxy, "proxy", "", "调试用：HTTP 代理地址，如 http://127.0.0.1:8888（抓包）")
	fs.BoolVar(&cf.insecure, "insecure", false, "调试用：忽略不可信的 HTTPS 证书")
	return cf
}

// openSession 按 cf 装配并登录一个 app.Session：构造【一个】gtrv 远程求解器（两端复用：游戏服
// 风控经 app 注入、bilibili 登录经 bsdk 注入），解析凭据（直传或账密冷启动），Login。返回非 0
// code 时已向 stderr 打印错误。
func openSession(ctx context.Context, cfg config.Config, name string, cf *credFlags, withMasterdata bool) (*app.Session, int) {
	solver := remote.New() // gtrv 远程求解器（pcrd 默认），游戏风控 + bilibili 登录两端共用

	opts := []app.Option{app.WithCaptchaSolver(solver)}
	if cf.proxy != "" {
		proxyURL, err := url.Parse(cf.proxy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: 代理地址无效: %v\n", name, err)
			return nil, 2
		}
		opts = append(opts, app.WithProxy(proxyURL))
	}
	if cf.insecure {
		opts = append(opts, app.WithInsecureTLS())
	}
	sess := app.NewSession(app.Dirs{Cache: cfg.Paths.Cache}, opts...)

	uid, accessKey, code := resolveIdentity(ctx, name, solver, cf)
	if code != 0 {
		return nil, code
	}
	if err := sess.Login(ctx, cf.channel, uid, accessKey, withMasterdata); err != nil {
		fmt.Fprintf(os.Stderr, "%s: 登录失败: %v\n", name, err)
		return nil, 1
	}
	return sess, 0
}

// resolveIdentity 得到游戏登录所需的 (uid, access_key)：给了 --username/--password 走 bilibili
// 账密【冷启动】（bsdklogin：账密→access_key，验证码经共享 solver 求解），否则用
// --uid/--access-key 直传。两种方式互斥、需其一。返回非 0 code 时已向 stderr 打印用法/错误。
func resolveIdentity(ctx context.Context, name string, solver *remote.Solver, cf *credFlags) (uid, accessKey string, code int) {
	switch {
	case cf.username != "" || cf.password != "":
		if cf.channel != app.ChannelBSDK {
			fmt.Fprintf(os.Stderr, "%s: 账密登录仅支持 bsdk 官服渠道（--channel=bsdk）\n", name)
			return "", "", 2
		}
		u, ak, err := bsdklogin.Login(ctx, cf.username, cf.password,
			bsdklogin.WithValidator(solver.Validator()), // bilibili 登录验证码
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: 账密登录失败: %v\n", name, err)
			return "", "", 1
		}
		return u, ak, 0
	case cf.uid != "" && cf.accessKey != "":
		return cf.uid, cf.accessKey, 0
	default:
		fmt.Fprintf(os.Stderr, "%s: 需要 --uid 与 --access-key，或 --username 与 --password\n", name)
		return "", "", 2
	}
}

// loginCommand 是 probe / inspect 的共同实现：登录后打印玩家档案。
// full=false 打印连通性摘要（probe，不接 masterdata）；full=true 打印完整玩家档案
// 并接入 masterdata（inspect：登录后按 manifest_ver 确保干净库并做示例查询）。
func loginCommand(cfg config.Config, name string, args []string, full bool) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cf := registerCredFlags(fs)
	var (
		debug   = fs.Bool("debug", false, "输出调试日志")
		timeout = fs.Duration("timeout", 60*time.Second, "整体超时")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *debug {
		logging.Setup(true)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// inspect（full）接入 masterdata：登录后按下发 res + manifest_ver 自动确保干净库；probe 不接。
	sess, code := openSession(ctx, cfg, name, cf, full)
	if code != 0 {
		return code
	}
	defer func() { _ = sess.Close() }()

	printPlayer(ctx, os.Stdout, sess, full)
	return 0
}

func printPlayer(ctx context.Context, w io.Writer, sess *app.Session, full bool) {
	d := sess.Player()
	var b strings.Builder
	fmt.Fprintln(&b, "登录成功 ✓")
	fmt.Fprintf(&b, "  昵称:       %s\n", d.UserName)
	fmt.Fprintf(&b, "  viewer_id:  %d\n", d.ViewerID)
	fmt.Fprintf(&b, "  等级:       %d\n", d.TeamLevel)
	fmt.Fprintf(&b, "  体力:       %d\n", d.Stamina)
	if full {
		fmt.Fprintf(&b, "  钻石:       %d（免费 %d）\n", d.Jewel.Total, d.Jewel.Free)
		fmt.Fprintf(&b, "  金币:       %d\n", d.Gold)
	}
	fmt.Fprintf(&b, "  资源版本:   %s（manifest %s）\n", d.ResVer, d.ManifestVer)
	fmt.Fprintf(&b, "  服务器时间: %s\n", time.Unix(sess.ServerTime(), 0).Format("2006-01-02 15:04:05"))
	if md := sess.Masterdata(); md != nil {
		printMasterdata(ctx, &b, md)
	}
	_, _ = io.WriteString(w, b.String())
}

// printMasterdata 打印母数据示例查询，验证干净库已接入并可查询。
func printMasterdata(ctx context.Context, b *strings.Builder, md app.Reader) {
	const sampleUnit = 100101 // 日和莉：稳定存在的角色 id，用作连通性示例
	n, err := md.Unit().Count(ctx)
	if err != nil {
		fmt.Fprintf(b, "  母数据:     查询失败: %v\n", err)
		return
	}
	name, err := md.Unit().Name(ctx, sampleUnit)
	if err != nil {
		name = fmt.Sprintf("(查不到 %d: %v)", sampleUnit, err)
	}
	fmt.Fprintf(b, "  母数据:     %d 个角色（示例 %d = %s）\n", n, sampleUnit, name)
}

// refreshCommand 是 `refresh`：免登录/免凭证把母数据刷新到最新版本并做连通性示例查询。
//
// 演示 masterdata 的无凭证刷新能力——无需 uid/access_key、无需等待登录，即自行握手
// （source_ini + get_maintenance_status）取最新 manifest_ver + res 并落库。
func refreshCommand(cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	var (
		channel = fs.String("channel", app.ChannelBSDK, "渠道：bsdk | qsdk")
		timeout = fs.Duration("timeout", 5*time.Minute, "整体超时（首次含母数据下载）")
		debug   = fs.Bool("debug", false, "输出调试日志")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *debug {
		logging.Setup(true)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	md, err := app.RefreshMasterdata(ctx, app.Dirs{Cache: cfg.Paths.Cache}, *channel, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "refresh: 刷新失败: %v\n", err)
		return 1
	}
	defer func() { _ = md.Close() }()

	var b strings.Builder
	b.WriteString("母数据已刷新到最新版本（免登录/免凭证）\n")
	printMasterdata(ctx, &b, md)
	_, _ = io.WriteString(os.Stdout, b.String())
	return 0
}

// runCommand 是 M3 `run`：登录后用运行器执行选定的自动化模块，打印结果。
// 无位置参数=运行全部模块；给出模块名=只运行这些；--list 仅列出可用模块。
func runCommand(cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	cf := registerCredFlags(fs)
	var (
		debug    = fs.Bool("debug", false, "输出调试日志")
		timeout  = fs.Duration("timeout", 60*time.Second, "整体超时")
		list     = fs.Bool("list", false, "列出可用模块/预设并退出")
		category = fs.String("category", "", "只运行某分类下的模块")
		preset   = fs.String("preset", "", "运行某批预设")
		cfgPath  = fs.String("config", "", "模块配置 JSON 文件（模块名→参数名→值）")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	registry := app.DefaultRegistry()
	if *list {
		printModules(os.Stdout, registry)
		return 0
	}
	if *debug {
		logging.Setup(true)
	}

	// 选择模块（优先级：--preset > --category > 位置参数 > 全部）。
	mods, code := selectModules(registry, *preset, *category, fs.Args())
	if code != 0 {
		return code
	}

	// 加载模块配置源（可选）。
	src, err := loadConfigSource(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run: 读取配置失败: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// 按需启用母数据：任一选中模块声明 NeedsMasterdata 时，登录后确保干净库就绪并打开查询面。
	sess, code := openSession(ctx, cfg, "run", cf, needsMasterdata(mods))
	if code != 0 {
		return code
	}
	defer func() { _ = sess.Close() }()

	// 进度经 stderr 实时打印（app.Observer 的 CLI 接线 / 端口 litmus）；结果仍走 stdout。
	results, err := sess.Run(ctx, app.TasksFor(mods, src), progressObserver(os.Stderr))
	printResults(os.Stdout, results)
	if err != nil { // 被取消/超时：只跑了 results 这些，非零退出
		fmt.Fprintf(os.Stderr, "run: 已取消，完成 %d 个任务: %v\n", len(results), err)
		return 1
	}
	for _, r := range results {
		if r.Status == app.StatusError {
			return 1 // 有模块失败时以非零退出，便于脚本判断
		}
	}
	return 0
}

// progressObserver 返回把任务级进度打到 w 的 app.Observer：开始时 “[i/n] ▶ 标题”、结束时附状态
// 字形。进度是外壳职责，回调只做即时打印（快、不阻塞、不 panic）——契约见 app.Observer。
func progressObserver(w io.Writer) app.Observer {
	return func(ev app.Event) {
		switch ev.Phase {
		case app.PhaseStarted:
			_, _ = fmt.Fprintf(w, "[%d/%d] ▶ %s\n", ev.Index+1, ev.Total, metaLabel(ev.Meta))
		case app.PhaseFinished:
			_, _ = fmt.Fprintf(w, "[%d/%d] %s %s\n", ev.Index+1, ev.Total, statusGlyph(ev.Result.Status), metaLabel(ev.Meta))
		}
	}
}

// metaLabel 取模块展示名，未知模块名（无 Title）时回落到稳定键。
func metaLabel(m app.Meta) string {
	if m.Title != "" {
		return m.Title
	}
	return m.Name
}

// needsMasterdata 报告选中模块中是否有任一声明依赖母数据。
func needsMasterdata(mods []app.Module) bool {
	for _, m := range mods {
		if m.Meta().NeedsMasterdata {
			return true
		}
	}
	return false
}

// selectModules 依优先级（--preset > --category > 位置参数 > 全部）挑选模块。
// 返回选中的模块与进程退出码（0=正常，非 0=选择出错，调用方应据此返回）。
func selectModules(registry *app.Registry, preset, category string, names []string) ([]app.Module, int) {
	switch {
	case preset != "":
		mods, ok, unknown := registry.Preset(preset)
		if !ok {
			fmt.Fprintf(os.Stderr, "run: 未知预设 %q（--list 查看）\n", preset)
			return nil, 2
		}
		if len(unknown) > 0 {
			fmt.Fprintf(os.Stderr, "run: 预设 %q 含未知模块 %v\n", preset, unknown)
			return nil, 2
		}
		return mods, 0
	case category != "":
		mods := registry.ByCategory(category)
		if len(mods) == 0 {
			fmt.Fprintf(os.Stderr, "run: 分类 %q 下无模块（--list 查看）\n", category)
			return nil, 2
		}
		return mods, 0
	case len(names) > 0:
		mods, unknown := registry.Select(names...)
		if len(unknown) > 0 {
			fmt.Fprintf(os.Stderr, "run: 未知模块 %v（--list 查看）\n", unknown)
			return nil, 2
		}
		return mods, 0
	default:
		return registry.All(), 0
	}
}

// loadConfigSource 从 JSON 文件加载模块配置源；path 为空返回 nil（全用默认）。
func loadConfigSource(path string) (app.Source, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var src app.Source
	if err := json.Unmarshal(data, &src); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", path, err)
	}
	return src, nil
}

// printModules 列出注册表中的可用模块（含参数）与批预设。
func printModules(w io.Writer, registry *app.Registry) {
	var b strings.Builder
	fmt.Fprintln(&b, "可用模块：")
	for _, m := range registry.All() {
		meta := m.Meta()
		fmt.Fprintf(&b, "  %-10s [%s] %s — %s\n", meta.Name, meta.Category, meta.Title, meta.Description)
		for _, p := range m.Params() {
			fmt.Fprintf(&b, "      · %s (%s，默认 %v%s) — %s\n", p.Name, p.Type, p.Default, boundsHint(p.Bounds), p.Description)
		}
	}
	if presets := registry.Presets(); len(presets) > 0 {
		fmt.Fprintln(&b, "批预设：")
		for _, p := range presets {
			fmt.Fprintf(&b, "  %-10s %s — %v\n", p.Name, p.Title, p.Modules)
		}
	}
	_, _ = io.WriteString(w, b.String())
}

// boundsHint 把参数边界渲染成简短提示（无边界返回空串）。
func boundsHint(b app.Bounds) string {
	switch {
	case len(b.Choices) > 0:
		return "，取值 " + strings.Join(b.Choices, "/")
	case b.Min != nil && b.Max != nil:
		return fmt.Sprintf("，范围 %d~%d", *b.Min, *b.Max)
	case b.Min != nil:
		return fmt.Sprintf("，≥%d", *b.Min)
	case b.Max != nil:
		return fmt.Sprintf("，≤%d", *b.Max)
	}
	return ""
}

// printResults 打印运行器返回的每个模块结果。
func printResults(w io.Writer, results []app.Result) {
	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "%s %s（%s）\n", statusGlyph(r.Status), r.Meta.Title, r.Meta.Name)
		for _, line := range r.Log {
			fmt.Fprintf(&b, "    %s\n", line)
		}
		if r.Err != nil {
			fmt.Fprintf(&b, "    错误: %v\n", r.Err)
		}
	}
	_, _ = io.WriteString(w, b.String())
}

func statusGlyph(s app.Status) string {
	switch s {
	case app.StatusOK:
		return "✓"
	case app.StatusSkip:
		return "–"
	default:
		return "✗"
	}
}

// usage 向 w 打印用法说明。
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `autopcr-cli — go-autopcr 测试命令行

用法:
  autopcr-cli <命令> [参数]

命令:
  version   显示版本与构建信息
  probe     验证传输/会话连通性（--uid --access-key | --username --password [--channel]）
  inspect   登录并打印玩家档案（昵称/等级/体力/钻石/金币）
  refresh   免登录/免凭证把母数据刷新到最新版本（[--channel] [--timeout]）
  run       登录后运行自动化模块（run --list 查看）
            选择：[模块名...] | --category <类> | --preset <预设>（无=全部）；--config <json> 传参
            注意：位置模块名须放在所有 --flag 之后（Go flag 遇首个非 flag 参数即停止解析）
  help      显示本帮助

凭据二选一：--uid + --access-key 直传；或 --username + --password 走 bilibili 账密登录（仅 bsdk）。
`)
}
