// Package exequip 汇集「EX 装备」域的自动化模块（彩装究极炼成等；对应 ref exequip.py）。
package exequip

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/cca2878/go-autopcr-core/internal/automation"
	"github.com/cca2878/go-autopcr-core/internal/client"
	apiexequip "github.com/cca2878/go-autopcr-core/internal/client/gameapi/exequip"
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	mdexequip "github.com/cca2878/go-autopcr-core/internal/client/masterdata/exequip"
)

// alcesUnlockQuest 是「究极炼成」的解锁任务（对应 ref alces_top 的 is_quest_cleared(11018002)）。
const alcesUnlockQuest = 11018002

// Register 登记本域全部模块。
func Register(r *automation.Registry) {
	r.Register(rainbowEnhance{})
}

func intPtr(n int) *int { return &n }

// rainbowEnhance 是「彩装究极炼成」模块（对应 ref ex_equip_rainbow_enchance）。
//
// 看属性＝列出彩装 id 与当前副属性；炼成＝按目标副属性反复重掷未锁槽（贪心：满级目标即锁、按
// 字典序权重取优）直至达标/资源耗尽；看概率暂未实现（本地统计已移交遥测侧，见 rc.Emit 每 roll 发射）。
type rainbowEnhance struct{}

func (rainbowEnhance) Meta() automation.Meta {
	return automation.Meta{
		Name:            "ex_equip_rainbow_enchance",
		Title:           "彩装究极炼成",
		Description:     "看属性/炼成彩装副属性。炼成按目标属性反复重掷未锁槽直至达标或PT耗尽；每次重掷经遥测发射。看概率暂未实现。",
		Category:        "EX装备",
		NeedsMasterdata: true,
	}
}

// Params 见 automation.Module。「彩装」与四个「炼成属性」「属性优先级」都不给静态候选——它们
// 依赖世界（前者是玩家库存、后者是母数据），由 Candidates 在登录后解析。
func (rainbowEnhance) Params() []automation.Param {
	return []automation.Param{
		{Name: "ex_equip_rainbow_enchance_action", Type: automation.ParamChoice, Default: "看属性",
			Description: "做什么", Bounds: automation.Bounds{Choices: []string{"看属性", "炼成", "看概率"}}},
		{Name: "ex_equip_rainbow_enchance_id", Type: automation.ParamChoice, Default: "0", Description: "彩装"},
		{Name: "ex_equip_rainbow_enchance_sub_status_1", Type: automation.ParamChoice, Default: "物贯", Description: "炼成属性1"},
		{Name: "ex_equip_rainbow_enchance_sub_status_2", Type: automation.ParamChoice, Default: "物贯", Description: "炼成属性2"},
		{Name: "ex_equip_rainbow_enchance_sub_status_3", Type: automation.ParamChoice, Default: "物贯", Description: "炼成属性3"},
		{Name: "ex_equip_rainbow_enchance_sub_status_4", Type: automation.ParamChoice, Default: "物贯", Description: "炼成属性4"},
		{Name: "ex_equip_rainbow_enhance_no_max_num", Type: automation.ParamInt, Default: 1,
			Description: "非满属性个数", Bounds: automation.Bounds{Min: intPtr(0), Max: intPtr(4)}},
		{Name: "ex_equip_rainbow_enhance_rank", Type: automation.ParamMultiChoice,
			Default: []string{"物贯", "法贯", "物攻", "魔攻"}, Description: "属性优先级"},
		{Name: "ex_equip_rainbow_enhance_pt_hold", Type: automation.ParamInt, Default: 10,
			Description: "保留pt数(w)", Bounds: automation.Bounds{Min: intPtr(0), Max: intPtr(1000)}},
	}
}

// Candidates 见 automation.Candidates：解析依赖世界的候选——「彩装」来自玩家库存（登录后才知道
// 有哪几件），副属性来自母数据。一次 LoadSnapshot 摊给全部六个参数。
//
// 只读已有的世界：母数据快照 + gc.Data() 的玩家态，不发网络请求。
func (rainbowEnhance) Candidates(ctx context.Context, gc client.GameClient) (map[string][]automation.Option, error) {
	md := gc.Masterdata()
	if md == nil {
		return nil, fmt.Errorf("彩装究极炼成需要母数据，但未启用")
	}
	snap, err := md.Exequip().LoadSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	// 副属性：母数据里出现过的全部属性（值即中文名，与 StatusByNameCh 对称）。
	statuses := snap.SubStatusCandidates()
	subs := make([]automation.Option, 0, len(statuses))
	for _, st := range statuses {
		name := mdexequip.ParamNameCh(st)
		subs = append(subs, automation.Option{Value: name, Label: name})
	}
	// 炼成目标额外可选「任意」＝不指定该槽（对应 ref 的 status 0）。
	targets := append([]automation.Option{{Value: "任意", Label: "任意"}}, subs...)

	out := map[string][]automation.Option{
		"ex_equip_rainbow_enchance_id":  rainbowOptions(gc, snap),
		"ex_equip_rainbow_enhance_rank": subs,
	}
	for i := 1; i <= 4; i++ {
		out[fmt.Sprintf("ex_equip_rainbow_enchance_sub_status_%d", i)] = targets
	}
	return out, nil
}

// rainbowOptions 是玩家持有的彩装候选：值＝serial_id，显示＝名称+当前副属性。按 serial_id 升序
// （玩家态是 map，迭代序不定）。无彩装→空切片，即「世界里当前没有可选项」，由 Run 的守卫报
// Skip("无彩装")。
func rainbowOptions(gc client.GameClient, snap *mdexequip.Snapshot) []automation.Option {
	equips := gc.Data().ExEquips
	serials := slices.Sorted(maps.Keys(equips))

	out := make([]automation.Option, 0, len(serials))
	for _, sid := range serials {
		e := equips[sid]
		if snap.Rarity(e.ExEquipmentID) != 5 {
			continue
		}
		out = append(out, automation.Option{
			Value: strconv.Itoa(sid),
			Label: fmt.Sprintf("%s %s", snap.ExEquipName(e.ExEquipmentID), subStatusStr(snap, e)),
		})
	}
	return out
}

func (rainbowEnhance) Run(ctx context.Context, gc client.GameClient, rc *automation.RunContext) error {
	md := gc.Masterdata()
	if md == nil {
		return fmt.Errorf("彩装究极炼成需要母数据，但未启用")
	}
	snap, err := md.Exequip().LoadSnapshot(ctx)
	if err != nil {
		return err
	}

	switch rc.String("ex_equip_rainbow_enchance_action") {
	case "看属性":
		return viewAttributes(gc, rc, snap)
	case "看概率":
		return automation.Skip("看概率暂未实现（本地统计已移交遥测侧）")
	case "炼成":
		return doEnhance(ctx, gc, rc, snap)
	default:
		return fmt.Errorf("未知操作")
	}
}

// viewAttributes 列出所有 5 星彩装的 id 与当前副属性（按 serial_id 升序稳定输出）。
func viewAttributes(gc client.GameClient, rc *automation.RunContext, snap *mdexequip.Snapshot) error {
	equips := gc.Data().ExEquips
	serials := slices.Sorted(maps.Keys(equips))

	var lines []string
	for _, sid := range serials {
		e := equips[sid]
		if snap.Rarity(e.ExEquipmentID) != 5 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%d: %s %s", sid, snap.ExEquipName(e.ExEquipmentID), subStatusStr(snap, e)))
	}
	if len(lines) == 0 {
		return automation.Skip("无彩装")
	}
	rc.Logf("%d件彩装:\n%s", len(lines), strings.Join(lines, "\n"))
	return nil
}

// doEnhance 执行究极炼成主流程（对应 ref action=='炼成' 分支）。
func doEnhance(ctx context.Context, gc client.GameClient, rc *automation.RunContext, snap *mdexequip.Snapshot) error {
	if !gc.Data().IsQuestCleared(alcesUnlockQuest) {
		return automation.Skip("究极炼成未解锁")
	}

	serialStr := rc.String("ex_equip_rainbow_enchance_id")
	serialID, err := strconv.Atoi(serialStr)
	if err != nil {
		return fmt.Errorf("彩装id非法")
	}
	equip, ok := gc.Data().ExEquips[serialID]
	if !ok || snap.Rarity(equip.ExEquipmentID) != 5 {
		return fmt.Errorf("彩装id不存在")
	}
	exID := equip.ExEquipmentID

	// 目标副属性（4 槽汇成 status→个数）。
	target := map[int]int{}
	for i := 1; i <= 4; i++ {
		st := mdexequip.StatusByNameCh(rc.String(fmt.Sprintf("ex_equip_rainbow_enchance_sub_status_%d", i)))
		if st != 0 {
			target[st]++
		}
	}
	// 校验目标属性均为该装备支持。
	var invalid []int
	for st := range target {
		if !snap.StatusSupported(exID, st) {
			invalid = append(invalid, st)
		}
	}
	if len(invalid) > 0 {
		slices.Sort(invalid)
		names := make([]string, len(invalid))
		for i, st := range invalid {
			names[i] = mdexequip.ParamNameCh(st)
		}
		return fmt.Errorf("炼成属性包含该装备不支持的属性: %s", strings.Join(names, ", "))
	}

	rankOrder := make([]int, 0)
	for _, n := range rc.Strings("ex_equip_rainbow_enhance_rank") {
		rankOrder = append(rankOrder, mdexequip.StatusByNameCh(n))
	}
	weight := computeWeight(target, rankOrder)

	r := &enhanceRun{gc: gc, rc: rc, snap: snap, serialID: serialID, exID: exID, target: target, weight: weight}

	// 已有待决定则先决定（须为同一彩装）。
	top, err := gc.Exequip().AlcesTop(ctx)
	if err != nil {
		return err
	}
	if top != nil {
		if top.SerialID != serialID {
			return fmt.Errorf("%d炼成属性待决定,请先自行决定", top.SerialID)
		}
		if _, err := r.decideAlces(ctx, top); err != nil {
			return err
		}
	}

	noMaxNum := rc.Int("ex_equip_rainbow_enhance_no_max_num")
	targetCnt := 0
	for _, c := range target {
		targetCnt += c
	}
	if noMaxNum > targetCnt {
		return fmt.Errorf("非满属性个数%d不能大于非任意的目标属性个数%d", noMaxNum, targetCnt)
	}

	alcesCost, err := gc.Masterdata().Exequip().AlcesCost(ctx)
	if err != nil {
		return err
	}
	ptHold := rc.Int("ex_equip_rainbow_enhance_pt_hold")

	rc.Logf("当前彩装属性 %d: %s %s", serialID, snap.ExEquipName(exID), subStatusStr(snap, equip))

	consumeCnt := map[mdexequip.ItemKey]int{}
	execCnt := 0
	lastLock := 0

	for {
		achMax, ach := r.getAchieved()
		if achMax >= targetCnt-noMaxNum && ach >= targetCnt {
			rc.Logf("彩装炼成属性已达成目标")
			break
		}

		lockCnt, err := r.doLock(ctx)
		if err != nil {
			return err
		}
		if lastLock != lockCnt {
			rc.Logf("L 锁定属性个数%d -> %d", lastLock, lockCnt)
			lastLock = lockCnt
		}

		pt := gc.Data().GetInventory(mdexequip.RainbowEnhancePt.Type, mdexequip.RainbowEnhancePt.ID)
		if pt <= ptHold*10000 {
			rc.Logf("彩装究极炼成PT%d<=%d，停止炼成", pt, ptHold*10000)
			break
		}

		// 计算本次材料消耗（随锁定数放大），不足即停。按键排序保证输出确定。
		toConsume := map[mdexequip.ItemKey]int{}
		stop := false
		for _, consume := range sortedItemKeys(alcesCost) {
			cost := alcesCost[consume] * (lockCnt + 1)
			toConsume[consume] = cost
			if cur := gc.Data().GetInventory(consume.Type, consume.ID); cur < cost {
				rc.Logf("E %s数量%d<%d，无法进行究极炼成", snap.ItemName(consume.ID), cur, cost)
				stop = true
			}
		}
		if stop {
			break
		}
		for k, v := range toConsume {
			consumeCnt[k] += v
		}

		pending, err := gc.Exequip().AlcesExec(ctx, serialID, pt, int(gc.Data().Gold))
		if err != nil {
			return err
		}
		accept, err := r.decideAlces(ctx, pending)
		if err != nil {
			return err
		}
		execCnt++

		verb := "R 放弃"
		if accept {
			verb = "A 接受"
		}
		rc.Logf("%s炼成属性: %s", verb, subStatusStrEntries(snap, exID, entriesFromPending(pending.SubStatus)))
	}

	if execCnt > 0 {
		rc.Logf("共进行了%d次究极炼成，消耗了：", execCnt)
		for _, k := range sortedItemKeys(consumeCnt) {
			rc.Logf("  %s x %d", snap.ItemName(k.ID), consumeCnt[k])
		}
	}
	final := gc.Data().ExEquips[serialID]
	rc.Logf("最终彩装属性 %d: %s %s", serialID, snap.ExEquipName(exID), subStatusStr(snap, final))
	return nil
}

// computeWeight 按优先级把 status 编码为加权值（对应 ref 的 base*=30 字典序编码）：优先级由低到高
// 逐个赋当前 base 后 ×30，使高优先级属性一档即压过低优先级满值；目标属性统一取最终 base（最高）。
func computeWeight(target map[int]int, rankOrder []int) map[int]int {
	weight := map[int]int{}
	base := 1
	for i := len(rankOrder) - 1; i >= 0; i-- {
		key := rankOrder[i]
		if _, inTarget := target[key]; !inTarget {
			weight[key] += base
			base *= 30
		}
	}
	for key := range target {
		weight[key] += base
	}
	return weight
}

// enhanceRun 承载一次炼成的共享状态与动作（对应 ref 模块的实例方法 + self.weight/target）。
type enhanceRun struct {
	gc       client.GameClient
	rc       *automation.RunContext
	snap     *mdexequip.Snapshot
	serialID int
	exID     int
	target   map[int]int
	weight   map[int]int
	rollIdx  int // 本次会话累计观测到的 roll 次数（遥测 roll_index）
}

// doLock 把已满级(step5)且仍需的目标属性槽锁定，其余解锁；返回锁定数（对应 ref do_lock）。
func (r *enhanceRun) doLock(ctx context.Context) (int, error) {
	currentMax := map[int]int{}
	lockCnt := 0
	for _, st := range r.gc.Data().ExEquips[r.serialID].SubStatus {
		toLock := false
		if currentMax[st.Status] < r.target[st.Status] && st.Step == 5 {
			currentMax[st.Status]++
			toLock = true
			lockCnt++
		}
		if toLock != st.IsLock {
			if err := r.gc.Exequip().AlcesLockSlot(ctx, r.serialID, st.SlotNumber, toLock); err != nil {
				return 0, err
			}
		}
	}
	return lockCnt, nil
}

// decideAlces 决定采纳/放弃一份待决定炼成数据（对应 ref decide_alces）：命中新的满级目标即采纳，
// 否则按加权字典序更优才采纳。每份待决定数据均经 rc.Emit 发射遥测。
func (r *enhanceRun) decideAlces(ctx context.Context, pending *apiexequip.AlcesPending) (bool, error) {
	r.emitRoll(pending)

	accept := false
	currentMax := map[int]int{}
	for _, st := range pending.SubStatus {
		if st.IsLock {
			currentMax[st.Status]++
			continue
		}
		if currentMax[st.Status] < r.target[st.Status] && st.Step == 5 {
			currentMax[st.Status]++
			accept = true
		}
	}

	if !accept {
		currentScore := 0
		for _, s := range r.gc.Data().ExEquips[pending.SerialID].SubStatus {
			currentScore += s.Step * r.weight[s.Status]
		}
		nxtScore := 0
		for _, s := range pending.SubStatus {
			nxtScore += s.Step * r.weight[s.Status]
		}
		if nxtScore > currentScore {
			accept = true
		}
	}

	if accept {
		return true, r.gc.Exequip().AlcesFixResult(ctx, pending.SerialID)
	}
	return false, r.gc.Exequip().AlcesCancelResult(ctx, pending.SerialID)
}

// getAchieved 统计已达成的目标属性数（含满级子计数；对应 ref get_achived_sub_status_cnt）。
func (r *enhanceRun) getAchieved() (achievedMax, achieved int) {
	current := map[int]int{}
	maxc := map[int]int{}
	for _, st := range r.gc.Data().ExEquips[r.serialID].SubStatus {
		current[st.Status]++
		if st.Step == 5 {
			maxc[st.Status]++
		}
	}
	for status, want := range r.target {
		achieved += min(current[status], want)
		achievedMax += min(maxc[status], want)
	}
	return achievedMax, achieved
}

// emitRoll 向遥测采集缝发射一次重掷观测（每槽 status/step/lock + 装备/序列上下文）。
func (r *enhanceRun) emitRoll(pending *apiexequip.AlcesPending) {
	r.rollIdx++
	subs := make([]map[string]any, len(pending.SubStatus))
	for i, s := range pending.SubStatus {
		subs[i] = map[string]any{"slot": s.SlotNumber, "status": s.Status, "step": s.Step, "is_lock": s.IsLock}
	}
	r.rc.Emit("alces_roll", map[string]any{
		"ex_equipment_id": r.exID,
		"serial_id":       pending.SerialID,
		"roll_index":      r.rollIdx,
		"sub_status":      subs,
	})
}

func subStatusStr(snap *mdexequip.Snapshot, e gamestate.ExEquip) string {
	return subStatusStrEntries(snap, e.ExEquipmentID, entriesFromState(e.SubStatus))
}

func subStatusStrEntries(snap *mdexequip.Snapshot, exID int, entries []mdexequip.SubStatusEntry) string {
	return snap.SubStatusStr(exID, entries)
}

func entriesFromState(subs []gamestate.ExEquipSubStatus) []mdexequip.SubStatusEntry {
	out := make([]mdexequip.SubStatusEntry, len(subs))
	for i, s := range subs {
		out[i] = mdexequip.SubStatusEntry{Status: s.Status, Step: s.Step}
	}
	return out
}

func entriesFromPending(subs []apiexequip.SubStatus) []mdexequip.SubStatusEntry {
	out := make([]mdexequip.SubStatusEntry, len(subs))
	for i, s := range subs {
		out[i] = mdexequip.SubStatusEntry{Status: s.Status, Step: s.Step}
	}
	return out
}

// sortedItemKeys 返回按 (类型, id) 升序的库存键——消耗清单要稳定输出，不能泄漏 map 迭代序。
func sortedItemKeys(m map[mdexequip.ItemKey]int) []mdexequip.ItemKey {
	return slices.SortedFunc(maps.Keys(m), func(a, b mdexequip.ItemKey) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}
