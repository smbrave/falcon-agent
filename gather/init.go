package gather

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/open-falcon/common/model"
)

const (
	UNKNOWN = 0
	ENABLE  = 1
	DISABLE = 2
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

func (gs *GatherStat) Rest() {
	gs.Total = 0
	gs.Counter = 0
	gs.Max = 0
	gs.Min = 100000000
}

type GatherItem struct {
	Enable int      `json:"enable"` //是否启用
	Metric string   `json:"metric"` //指标名称
	Rule   string   `json:"rule"`   //采集正则表达式
	Tags   []string `json:"tags"`   //key=正则表达式
	Type   string   `json:"type"`   //上报方式 默认GAUGE 支持MAX、MIN、SUM、AVG
}

type GatherFile struct {
	Enable     int          `json:"enable"`      //是否启用
	GatherStep int          `json:"gather_step"` //采集间隔 默认10s
	ReportStep int          `json:"report_step"` //上报间隔 默认60s
	File       string       `json:"file"`        //文件名
	Format     string       `json:"foramt"`      //时间格式 默认:2006-01-02 15:04:05
	Items      []GatherItem `json:"items"`       //采集项目
}

type Config struct {
	Enable int          `json:"enable"` //是否启用
	Files  []GatherFile `json:"files"`  //需要采集的文件
}

var config Config

func Init(cfgFile string) {
	cfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(cfg), &config)
	if err != nil {
		panic(err)
	}

	if config.Enable == UNKNOWN {
		config.Enable = ENABLE
	}

	//设置默认值和校验
	for i, gf := range config.Files {
		if gf.Enable == UNKNOWN {
			config.Files[i].Enable = ENABLE
		}
		if gf.ReportStep == 0 {
			config.Files[i].ReportStep = 10 //上报间隔
		}

		if gf.GatherStep == 0 {
			config.Files[i].GatherStep = 1 //采集周期
		}

		if gf.Format == "" {
			config.Files[i].Format = "2006-01-02 15:04:05"
		}

		for j, item := range config.Files[i].Items {

			//校验正则表达式
			if item.Rule != "" {
				_, err = regexp.Compile(item.Rule)
				if err != nil {
					panic(err)
				}
			}

			//校验正则表达式
			for _, tag := range item.Tags {
				fields := strings.SplitN(tag, "=", 2)
				_, err = regexp.Compile(fields[1])
				if err != nil {
					panic(err)
				}
			}

			if item.Enable == UNKNOWN {
				config.Files[i].Items[j].Enable = ENABLE
			}

			if item.Type == "" {
				config.Files[i].Items[j].Type = "GAUGE" //上报方式
			}
		}
	}

}