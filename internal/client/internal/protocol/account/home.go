package account

import (
	"net/url"

	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
)

var (
	urlHomeIndex     = protocol.MustRelURL("home/index")
	urlDailyTaskTop  = protocol.MustRelURL("daily_task/top")
	urlUnitRoleGacha = protocol.MustRelURL("unit_role/gacha_index")
)

// HomeIndexRequest 拉取主页索引（含任务通关状态、支线通关列表等）。
type HomeIndexRequest struct {
	protocol.RequestBase
	MessageID   int   `msgpack:"message_id" json:"message_id"`
	TipsIDList  []int `msgpack:"tips_id_list" json:"tips_id_list"`
	IsFirst     int   `msgpack:"is_first" json:"is_first"`
	GoldHistory int   `msgpack:"gold_history" json:"gold_history"`
}

func (*HomeIndexRequest) URL() *url.URL { return urlHomeIndex }

// DailyTaskTopRequest 拉取日常任务首页。权威客户端在「刚通关普通 8-1」时用它解锁日常任务，
// 故登录序列按同样条件发出（字段取值复刻权威客户端）。
type DailyTaskTopRequest struct {
	protocol.RequestBase
	SettingAlchemyCount      int `msgpack:"setting_alchemy_count" json:"setting_alchemy_count"`
	IsCheckByTermNormalGacha int `msgpack:"is_check_by_term_normal_gacha" json:"is_check_by_term_normal_gacha"`
}

func (*DailyTaskTopRequest) URL() *url.URL { return urlDailyTaskTop }

// DailyTaskTopResponse 无需读取的字段由解码器忽略。
type DailyTaskTopResponse struct {
	protocol.ResponseBase
}

// UnitRoleGachaIndexRequest 拉取角色扮演转蛋首页；权威客户端登录序列的最后一步。
type UnitRoleGachaIndexRequest struct {
	protocol.RequestBase
}

func (*UnitRoleGachaIndexRequest) URL() *url.URL { return urlUnitRoleGacha }

// UnitRoleGachaIndexResponse 无需读取的字段由解码器忽略。
type UnitRoleGachaIndexResponse struct {
	protocol.ResponseBase
}

// UserQuestInfo 是一条任务的玩家进度（此处取解锁判定所需字段）。
// clear_flg>0 表示该任务已通关（用于剧情解锁门禁）。
type UserQuestInfo struct {
	QuestID    int `msgpack:"quest_id" json:"quest_id"`
	ClearFlg   int `msgpack:"clear_flg" json:"clear_flg"`
	ResultType int `msgpack:"result_type" json:"result_type"`
}

// TrainingQuestCount 是训练（探索）扫荡次数（exp/gold 两类）。
type TrainingQuestCount struct {
	GoldQuest int `msgpack:"gold_quest" json:"gold_quest"`
	ExpQuest  int `msgpack:"exp_quest" json:"exp_quest"`
}

// HomeIndexResponse 携带任务通关状态、支线通关列表与训练扫荡次数（其余字段由解码器忽略）。
type HomeIndexResponse struct {
	protocol.ResponseBase
	QuestList               []UserQuestInfo     `msgpack:"quest_list" json:"quest_list"`
	ClearedBywayQuestIDList []int               `msgpack:"cleared_byway_quest_id_list" json:"cleared_byway_quest_id_list"`
	TrainingQuestCount      *TrainingQuestCount `msgpack:"training_quest_count" json:"training_quest_count"`
	TrainingQuestMaxCount   *TrainingQuestCount `msgpack:"training_quest_max_count" json:"training_quest_max_count"`
}
