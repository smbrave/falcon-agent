package gather

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/open-falcon/common/model"
)

const (
	ENABLE  = 1
	DISABLE = 0
)

var (
	localtion *time.Location
)

func init() {
	localtion, _ = time.LoadLocation("Local")
}

type LogInfo struct {
	LogLine *string
	LogTime *time.Time
}

type GatherWorker struct {
	GFile      *GatherFile
	ReadPos    int64
	DataChan   []chan *LogInfo
	MetricChan chan *model.MetricValue
}

type GatherStat struct {
	Total   float64
	Counter float64
	Max     float64
	Min     float64
}

type GatherItem struct {
	Metric string   `json:"metric"` //指标名称
	Rule   string   `json:"match"`  //采集正则表达式
	Tags   []string `json:"tags"`   //key=正则表达式
	Type   string   `json:"type"`   //上报方式 默认GAUGE 支持MAX、MIN、SUM、AVG
}

type GatherFile struct {
	ReportStep int           `json:"step"`   //上报间隔 默认60s
	File       string        `json:"file"`   //文件名
	Format     string        `json:"format"` //时间格式 默认:2006-01-02 15:04:05
	Items      []*GatherItem `json:"rules"`  //采集项目
}

type Config struct {
	Files []*GatherFile `json:"files"` //需要采集的文件
}

type ConfigResponse struct {
	Errno  int    `json:"errno"`
	Errmsg string `json:"errmsg"`
	Data   struct {
		Monitor []*GatherFile `json:"monitor"`
		Cmd     string        `json:"cmd"`
	} `json:"data"`
}

func (gs *GatherStat) Rest() {
	gs.Total = 0
	gs.Counter = 0
	gs.Max = 0
	gs.Min = 100000000
}

func Default(monitor []*GatherFile) error {

	var err error
	//设置默认值和校验
	for _, gf := range monitor {

		if gf.ReportStep == 0 {
			gf.ReportStep = 10 //上报间隔
		}

		if gf.Format == "" {
			gf.Format = "2006-01-02 15:04:05"
		}

		for j, item := range gf.Items {

			//校验正则表达式
			if item.Rule != "" {
				_, err = regexp.Compile(item.Rule)
				if err != nil {
					fmt.Println(item.Rule, "error:", err.Error())
					return err
				}
			}

			//校验正则表达式
			for _, tag := range item.Tags {
				fields := strings.SplitN(tag, "=", 2)
				_, err = regexp.Compile(fields[1])
				if err != nil {
					fmt.Println(item.Rule, "error:", err.Error())
					return err
				}
			}

			if item.Type == "" {
				gf.Items[j].Type = "GAUGE" //上报方式
			}
		}
	}
	return nil
}
