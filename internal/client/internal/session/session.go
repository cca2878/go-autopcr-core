// Package session 实现基于 AccessKey 的游戏服登录序列。
//
// 对应原 Python 项目 core/sessionmgr.py 的登录编排（去掉了 token 文件缓存——纯库
// 不持久化状态）。Login 只负责「按序发出请求 + 控制流校验」，返回 error；各响应
// 的数据由上层安装的折叠中间件（gameclient）落入玩家状态，故本包不依赖 gamestate。
package session

import (
	"context"
	"math/rand"

	"github.com/cca2878/go-autopcr-core/internal/client/credential"
	"github.com/cca2878/go-autopcr-core/internal/client/gameerr"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/discovery"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/account"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/transport"
)

// Login 执行完整登录序列：
//
//	source_ini/index → get_maintenance_status → tool/sdk_login →
//	check/game_start → load/index → home/index
//
// 权威客户端在此之后还会发 daily_task/top（普通 8-1 已领取时）与 unit_role/gacha_index。
// 本库【有意不发】：实测两者只回 task_list 与 exec_count/gacha_level，没有任何本库模块消费的
// 状态，发它们纯属每次登录多两发。若将来移植依赖这些状态的模块，需要连同这两步一起补回。
//
// 风控（is_risk）未通过验证码时返回 gameerr.RiskError（未注入求解器即硬失败）；
// 未过教程返回 PanicError。
func Login(ctx context.Context, c *transport.Client, cred credential.Credential) error {
	// 1-2) 免凭证发现握手：服务器列表 + 维护/版本状态（响应经折叠中间件落入玩家状态）。
	disc, err := discovery.Discover(ctx, c)
	if err != nil {
		return err
	}
	if disc.RequiredManifestVer != "" {
		c.SetHeader("MANIFEST-VER", disc.RequiredManifestVer)
	}

	// 3) SDK 登录（AccessKey 四要素）
	uid, accessKey, err := cred.Login(ctx)
	if err != nil {
		return err
	}
	loginReq := &sdk.ToolSdkLoginRequest{
		UID:       uid,
		AccessKey: accessKey,
		Platform:  cred.PlatformID(),
		ChannelID: cred.ChannelID(),
	}
	loginResp, err := transport.Call[sdk.ToolSdkLoginResponse](ctx, c, loginReq)
	if err != nil {
		return err
	}
	if loginResp.IsRisk {
		// 把首个风控响应的未知载荷带入 passRisk——无求解器（mobile）场景下它就是最终透出的载荷。
		if err := passRisk(ctx, c, cred, uid, accessKey, loginResp.Extra); err != nil {
			return err
		}
	}

	// 4) 校验游戏启动
	startReq := &sdk.CheckGameStartRequest{
		AppType:      0,
		CampaignData: "",
		CampaignUser: rand.Intn(100001) &^ 1, // 随机偶数，复刻原项目
	}
	start, err := transport.Call[sdk.CheckGameStartResponse](ctx, c, startReq)
	if err != nil {
		return err
	}
	if !start.NowTutorial {
		return gameerr.Panic("账号未过完教程")
	}

	// 5) 首页索引：玩家档案（昵称/等级/体力/钻石/金币）经折叠中间件落入状态
	if _, err := transport.Call[account.LoadIndexResponse](ctx, c, &account.LoadIndexRequest{Carrier: "OPPO"}); err != nil {
		return err
	}

	// 6) 主页索引：任务通关/支线状态（剧情解锁门禁等所需）经折叠中间件落入状态
	if _, err := transport.Call[account.HomeIndexResponse](ctx, c, &account.HomeIndexRequest{MessageID: 1, IsFirst: 1, TipsIDList: []int{}}); err != nil {
		return err
	}

	return nil
}

// maxRiskAttempts 是触发风控后允许的验证码重试轮数（复刻原项目上限）。
const maxRiskAttempts = 5

// passRisk 处理 tool/sdk_login 返回 is_risk 的风控：循环「求解验证码 → 带票据重登」，
// 直到某轮登录不再 is_risk。求解由 credential 注入的验证码求解器完成（核心不自带求解器——
// 求解能力由外壳经 accesskey.WithCaptchaSolver 注入）；重登请求在四要素之外补齐
// challenge/validate/seccode 等 geetest 票据（复刻原项目 sessionmgr 的重提交字段）。
//
// 求解失败（含未注入求解器 → captcha.ErrNoSolver）即【硬失败】：不发重登，返回 distinct
// 的 gameerr.RiskError（Unwrap 保留成因，便于外壳 errors.Is/As 诊断与数据采集）。这是
// 有意的设计——is_risk 极罕见且行为不明，暂不在核心内投机求解（见架构决策）。
// payload 为触发本次风控的响应中未建模字段的快照，随每轮重登刷新为最近一次风控响应的载荷，
// 最终随 RiskError 透出（供数据采集）。
func passRisk(ctx context.Context, c *transport.Client, cred credential.Credential, uid, accessKey string, payload map[string]any) error {
	for i := range maxRiskAttempts {
		res, err := cred.DoCaptcha(ctx)
		if err != nil {
			return gameerr.Risk(i, err, payload)
		}
		req := &sdk.ToolSdkLoginRequest{
			UID:         uid,
			AccessKey:   accessKey,
			Platform:    cred.PlatformID(),
			ChannelID:   cred.ChannelID(),
			Challenge:   &res.Challenge,
			Validate:    &res.Validate,
			Seccode:     &res.Seccode,
			CaptchaType: strptr("1"),
			ImageToken:  strptr(""),
			CaptchaCode: strptr(""),
		}
		resp, err := transport.Call[sdk.ToolSdkLoginResponse](ctx, c, req)
		if err != nil {
			return err
		}
		if !resp.IsRisk {
			return nil // 风控解除
		}
		payload = resp.Extra // 刷新为最近一次风控响应的载荷
	}
	return gameerr.Risk(maxRiskAttempts, nil, payload)
}

func strptr(s string) *string { return &s }
