package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
)

// maxMissingEmblemNames 是报告里最多列出的称号名数量（其余以计数概括，避免刷屏）。
const maxMissingEmblemNames = 50

// missingEmblem 报告【尚未获得的称号】（只读；对应 ref missing_emblem）。
type missingEmblem struct{}

func (missingEmblem) Meta() automation.Meta {
	return automation.Meta{
		Name:            "missing_emblem",
		Title:           "查缺称号",
		Description:     "报告尚未获得的称号及其达成条件（只读）",
		Category:        "查询",
		NeedsMasterdata: true,
	}
}

func (missingEmblem) Params() []automation.Param { return nil }

func (missingEmblem) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("查缺称号需要母数据，但未启用")
	}
	all, err := md.Emblem().AllEmblems(ctx)
	if err != nil {
		return err
	}
	owned, err := gc.Emblem().OwnedEmblemIDs(ctx)
	if err != nil {
		return err
	}

	var names []string
	missing := 0
	for _, e := range all {
		if _, has := owned[e.ID]; has {
			continue
		}
		missing++
		if len(names) < maxMissingEmblemNames {
			if e.Description != "" {
				names = append(names, fmt.Sprintf("%s（%s）", e.Name, e.Description))
			} else {
				names = append(names, e.Name)
			}
		}
	}

	if missing == 0 {
		return automation.Skip("全称号玩家！没有缺少的称号")
	}
	rc.Logf("缺少称号：%d", missing)
	rc.Logf("%s", strings.Join(names, "\n"))
	if missing > len(names) {
		rc.Logf("……另有 %d 个未列出", missing-len(names))
	}
	return nil
}
