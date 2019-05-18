package gatherv2

import (
	"encoding/json"

	"github.com/hpcloud/tail"
)

const (
	///GAUGE 支持MAX、MIN、SUM、AVG
	TYPE_GAUGE = "GAUGE"
	TYPE_MAX   = "MAX"
	TYPE_MIN   = "MIN"
	TYPE_SUM   = "SUM"
	TYPE_AVG   = "AVG"
)

type GatherStat struct {
	Total   float64
	Counter float64
	Max     float64
	Min     float64
}

func (gs *GatherStat) Rest() {
	gs.Total = 0
	gs.Counter = 0
	gs.Max = 0
	gs.Min = 100000000
}

type RuleConf struct {
	Metric string   `json:"metric"` //指标名称
	Match  string   `json:"match"`  //采集正则表达式
	Tags   []string `json:"tags"`   //key=正则表达式
	Type   string   `json:"type"`   //上报方式 默认GAUGE 支持MAX、MIN、SUM、AVG
}

type FileConf struct {
	Step   int         `json:"step"`   //上报间隔 默认60s
	File   string      `json:"file"`   //文件名
	Format string      `json:"format"` //时间格式 默认:2006-01-02 15:04:05
	Rules  []*RuleConf `json:"rules"`  //采集项目
}

type AgentData struct {
	Files  []*FileConf `json:"files"`
	Cmd    string      `json:"cmd"`
	Update int64       `json:"update"`
}
type ConfigResponse struct {
	Errno  int        `json:"errno"`
	Errmsg string     `json:"errmsg"`
	Data   *AgentData `json:"data"`
}

type IObserver interface {
	Notify(*tail.Line)
	Start() error
	Stop() error
	Run()
}

func ObjectString(obj interface{}) string {
	res, _ := json.Marshal(obj)
	return string(res)
}
