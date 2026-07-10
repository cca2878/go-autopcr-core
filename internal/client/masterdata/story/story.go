// Package story 是母数据「剧情」域的只读查询（剧情列表、解锁时间等；随需增量）。
package story

import (
	"context"
	"database/sql"
	"time"

	"github.com/cca2878/go-autopcr/internal/client/masterdata/mddb"
)

const (
	// birthdayStoryGroupID 是生日剧情所属的 story_group_id（对应 ref db.birthday_story）。
	birthdayStoryGroupID = 4010
	// 主线剧情 story_id 区间（对应 ref db.main_story：story_detail 中 [mainStoryLo, mainStoryHi)）。
	mainStoryLo = 2000000
	mainStoryHi = 3000000
	// 角色好感剧情 story_id 区间（对应 ref db.unit_story：story_detail 中 [unitStoryLo, unitStoryHi)）。
	unitStoryLo = 1000000
	unitStoryHi = 2000000
	// MagicalGirlGroupID 是「魔姬」剧情的 story_group_id，读取判定中按 ref 忽略。
	MagicalGirlGroupID = 1255
)

// Story 是一条剧情（此处取报告/解锁判定所需字段）。
type Story struct {
	StoryID       int
	PreStoryID    int       // 前置剧情 id（0＝无前置）
	UnlockQuestID int       // 解锁所需通关的任务 id（0＝无门禁）
	StartTime     time.Time // 解锁时间（服务器时间达到后方可阅读）
	Title         string    // 展示名（title 为空时回退 sub_title）
}

// UnitStory 是一条角色好感剧情（此处取好感解锁判定所需字段）。
type UnitStory struct {
	StoryID          int
	StoryGroupID     int       // ＝角色 chara_id，用于查玩家该角色好感等级
	PreStoryID       int       // 前置剧情 id（0＝无前置）
	PreStoryID2      int       // 第二前置剧情 id（0＝无）
	LoveLevel        int       // 解锁所需好感等级
	ReadProcessFlag  bool      // 是否为可正常阅读的剧情
	ForceUnlockTime  time.Time // 到此时间即使前置未读也强制解锁
	ForceUnlockTime2 time.Time // 第二前置的强制解锁时间
	StartTime        time.Time
	EndTime          time.Time
	Title            string
}

// API 是剧情域查询契约（随功能在本包内累加）。
type API interface {
	// BirthdayStories 返回全部生日剧情（按 story_id 升序），供跨玩家已读状态判定可阅读项。
	BirthdayStories(ctx context.Context) ([]Story, error)
	// MainStories 返回主线剧情 + 支线剧情（按 story_id 升序，含解锁任务门禁），供可读判定。
	MainStories(ctx context.Context) ([]Story, error)
	// UnitStories 返回全部角色好感剧情（按 story_id 升序，含好感等级门禁），供可读判定。
	UnitStories(ctx context.Context) ([]UnitStory, error)
}

// Impl 是 API 的实现，只持有低层只读 DB 句柄。
type Impl struct {
	db *mddb.DB
}

// New 构造剧情域查询实现。
func New(db *mddb.DB) *Impl { return &Impl{db: db} }

func (a *Impl) BirthdayStories(ctx context.Context) ([]Story, error) {
	rows, err := a.db.QueryContext(ctx,
		"SELECT story_id, pre_story_id, unlock_quest_id, start_time, title, sub_title FROM story_detail WHERE story_group_id = ? ORDER BY story_id",
		birthdayStoryGroupID)
	if err != nil {
		return nil, err
	}
	return scanStories(rows)
}

func (a *Impl) MainStories(ctx context.Context) ([]Story, error) {
	// 主线（story_detail 指定区间）+ 支线（byway_story_detail）合并，按 story_id 升序——
	// 保证前置剧情（更小 id）先于续篇被处理，链式解锁判定成立。byway 无 sub_title 列，补空串占位。
	rows, err := a.db.QueryContext(ctx, `
		SELECT story_id, pre_story_id, unlock_quest_id, start_time, title, sub_title
		  FROM story_detail WHERE story_id >= ? AND story_id < ?
		UNION ALL
		SELECT story_id, pre_story_id, unlock_quest_id, start_time, title, '' AS sub_title
		  FROM byway_story_detail
		ORDER BY story_id`,
		mainStoryLo, mainStoryHi)
	if err != nil {
		return nil, err
	}
	return scanStories(rows)
}

func (a *Impl) UnitStories(ctx context.Context) ([]UnitStory, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT story_id, story_group_id, pre_story_id, pre_story_id_2, love_level, read_process_flag,
		       force_unlock_time, force_unlock_time_2, start_time, end_time, title
		  FROM story_detail WHERE story_id >= ? AND story_id < ? ORDER BY story_id`,
		unitStoryLo, unitStoryHi)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []UnitStory
	for rows.Next() {
		var (
			s                                                    UnitStory
			readFlag                                             int
			forceUnlock, forceUnlock2, startTime, endTime, title string
		)
		if err := rows.Scan(&s.StoryID, &s.StoryGroupID, &s.PreStoryID, &s.PreStoryID2, &s.LoveLevel, &readFlag,
			&forceUnlock, &forceUnlock2, &startTime, &endTime, &title); err != nil {
			return nil, err
		}
		for _, p := range []struct {
			src string
			dst *time.Time
		}{{forceUnlock, &s.ForceUnlockTime}, {forceUnlock2, &s.ForceUnlockTime2}, {startTime, &s.StartTime}, {endTime, &s.EndTime}} {
			t, err := ParseTime(p.src)
			if err != nil {
				return nil, err
			}
			*p.dst = t
		}
		s.ReadProcessFlag = readFlag != 0
		s.Title = title
		out = append(out, s)
	}
	return out, rows.Err()
}

// scanStories 从统一的 6 列结果（story_id, pre_story_id, unlock_quest_id, start_time, title,
// sub_title）读出剧情列表；title 为空时回退 sub_title。负责 Close 传入的 rows。
func scanStories(rows *sql.Rows) ([]Story, error) {
	defer func() { _ = rows.Close() }()
	var out []Story
	for rows.Next() {
		var (
			storyID, preStoryID, unlockQuestID int
			startTime, title, sub              string
		)
		if err := rows.Scan(&storyID, &preStoryID, &unlockQuestID, &startTime, &title, &sub); err != nil {
			return nil, err
		}
		t, err := ParseTime(startTime)
		if err != nil {
			return nil, err
		}
		if title == "" {
			title = sub
		}
		out = append(out, Story{StoryID: storyID, PreStoryID: preStoryID, UnlockQuestID: unlockQuestID, StartTime: t, Title: title})
	}
	return out, rows.Err()
}

// ParseTime 解析母数据里的时间；委托给共享实现（见 mddb.ParseTime）。
// 保留本域导出名以兼容既有调用方。
func ParseTime(s string) (time.Time, error) { return mddb.ParseTime(s) }
