package client

import (
	"context"
	"log/slog"
	"testing"

	"github.com/cca2878/go-autopcr-core/internal/client/credential/captcha"
)

// stubCred 是仅满足 transport.New / session.Login 所需的凭据打桩。
type stubCred struct{}

func (stubCred) Login(context.Context) (string, string, error) { return "u", "k", nil }
func (stubCred) Header() map[string]string                     { return map[string]string{} }
func (stubCred) APIRoot() string                               { return "https://test.example/" }
func (stubCred) PlatformID() string                            { return "2" }
func (stubCred) ChannelID() string                             { return "1" }
func (stubCred) DoCaptcha(context.Context) (*captcha.Result, error) {
	return &captcha.Result{}, nil
}

// 登录序列是权威全量源，故 loginSequence 必须'先清零再跑序列'。
//
// 用已取消的 ctx 让序列在第一发就失败，从而把"清零"与"折叠"分离开单独观察：序列一步没跑，
// 陈旧字段却已归零，说明 Reset 确实在 session.Login 之前。顺带锁住 loginSequence 声明的那条
// 语义——序列中途失败时状态停在空，而不是留半份旧数据。
//
// 另一半（清零后 maintenance 会在同一序列里重新填回版本/CDN 字段）由 session 包的
// TestLoginSequence 保证：它断言 get_maintenance_status 固定是序列第 2 步。
func TestLoginSequenceResetsBeforeSequence(t *testing.T) {
	g := New(stubCred{}, WithLogger(slog.New(slog.DiscardHandler))).(*client)

	// 上一轮遗留：已加入公会（由 foldLoadIndex 的 if 保护，退会后服务端不再下发）、
	// 点赞后置 1 的本地增量、以及母数据版本。
	g.state.ClanID = 999
	g.state.ClanLikeCount = 1
	g.state.ResVer = "stale"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Login(ctx); err == nil {
		t.Fatal("前置条件不成立：ctx 已取消，登录序列不该成功")
	}

	if g.state.ClanID != 0 {
		t.Errorf("ClanID=%d，陈旧值活过了重登（Reset 未在序列之前执行）", g.state.ClanID)
	}
	if g.state.ClanLikeCount != 0 {
		t.Errorf("ClanLikeCount=%d，本地增量活过了重登", g.state.ClanLikeCount)
	}
	if g.state.ResVer != "" {
		t.Errorf("ResVer=%q，序列失败时状态应停在空而非留半份旧数据", g.state.ResVer)
	}
}
