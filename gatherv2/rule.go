package gatherv2

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/open-falcon/agent/g"
	"github.com/open-falcon/common/model"
)

type RuleWatcher struct {
	FileWatcher *FileWatcher
	Rule        *RuleConf
	Lines       chan *tail.Line
	TagRegexp   map[string]*regexp.Regexp
	LineRegexp  *regexp.Regexp
	Exit        chan struct{}
}

func NewRuleWatcher(w *FileWatcher, rule *RuleConf) *RuleWatcher {
	return &RuleWatcher{
		FileWatcher: w,
		Rule:        rule,
		Lines:       make(chan *tail.Line, 100),
		Exit:        make(chan struct{}),
	}
}

//搜集数据
func (w *RuleWatcher) GatherData(value float64, tagKey string, tagsStat map[string]*GatherStat) {
	var stat *GatherStat = nil
	var ok bool

	//有tag的汇总一份总数据
	if tagKey != "" {
		if stat, ok = tagsStat[""]; !ok {
			stat = new(GatherStat)
			stat.Rest()
			tagsStat[""] = stat
		}
		stat.Counter += 1
		stat.Total += value
		if value > stat.Max {
			stat.Max = value
		}
		if value < stat.Min {
			stat.Min = value
		}
	}

	//添加到对应tag上
	if stat, ok = tagsStat[tagKey]; !ok {
		stat = new(GatherStat)
		stat.Rest()
		tagsStat[tagKey] = stat
	}

	stat.Counter += 1
	stat.Total += value
	if value > stat.Max {
		stat.Max = value
	}
	if value < stat.Min {
		stat.Min = value
	}
}

func (w *RuleWatcher) ParseLine(line *tail.Line) (float64, string, error) {
	value := float64(0)
	tag_key := ""

	lineMathch := w.LineRegexp.FindStringSubmatch(line.Text)
	if len(lineMathch) == 0 {
		return 0, "", fmt.Errorf("not match :%s", w.Rule.Match)
	}

	if len(lineMathch) >= 2 {
		value, _ = strconv.ParseFloat(lineMathch[1], 64)
	}
	tagsResult := make([]string, 0)
	for k, v := range w.TagRegexp {
		tagMatch := v.FindStringSubmatch(line.Text)
		if len(tagMatch) >= 2 {
			tagsResult = append(tagsResult, fmt.Sprintf("%s=%s", k, tagMatch[1]))
		} else {
			tagsResult = append(tagsResult, fmt.Sprintf("%s=", k))
		}
	}
	sort.Strings(tagsResult)
	if len(tagsResult) != 0 {
		tag_key = strings.Join(tagsResult, ",")
	}
	return value, tag_key, nil
}

func (w *RuleWatcher) ReportData(tagStat map[string]*GatherStat) {
	if len(tagStat) == 0 {
		return
	}
	tp := strings.ToUpper(w.Rule.Type)
	hostname, _ := g.Hostname()
	mertics := make([]*model.MetricValue, 0)
	for k, v := range tagStat {
		metric := new(model.MetricValue)
		metric.Metric = w.Rule.Metric
		switch tp {
		case TYPE_GAUGE:
			metric.Value = v.Counter
		case TYPE_SUM:
			metric.Value = v.Total
		case TYPE_MAX:
			metric.Value = v.Max
		case TYPE_MIN:
			metric.Value = v.Min
		case TYPE_AVG:
			metric.Value = v.Total / v.Counter
		default:
			metric.Value = v.Counter
		}

		metric.Tags = k
		metric.Step = int64(w.FileWatcher.MFile.Step)
		metric.Timestamp = time.Now().Unix()
		metric.Endpoint = hostname
		metric.Type = "GAUGE"
		mertics = append(mertics, metric)
	}

	var resp model.TransferResponse
	g.SendMetrics(mertics, &resp)
	fmt.Printf("=> metric:%s resp:%s\n", mertics[0].Metric, resp.String())
	for i, me := range mertics {
		log.Printf("==> idx:%d mertic:%v \n", i, me)
	}

}

func (w *RuleWatcher) Run() {
	ticker := time.NewTicker(time.Second * time.Duration(w.FileWatcher.MFile.Step))
	defer ticker.Stop()

	tagsStat := make(map[string]*GatherStat)
	for {
		select {
		case line := <-w.Lines: //收集数据统计
			value, tagKey, err := w.ParseLine(line)
			if err != nil {
				continue
			}
			w.GatherData(value, tagKey, tagsStat)

		case <-ticker.C: //定时上报数据
			w.ReportData(tagsStat)
			tagsStat = make(map[string]*GatherStat)

		case <-w.Exit: //退出
			goto exit
		}
	}
exit:
	log.Println("[INFO] file:", w.FileWatcher.MFile.File, "metric:", w.Rule.Metric, "stoped")
}

func (w *RuleWatcher) Start() error {
	var err error
	//标签正则表达
	w.TagRegexp = make(map[string]*regexp.Regexp)
	w.LineRegexp, err = regexp.Compile(w.Rule.Match)
	if err != nil {
		log.Println("[ERROR] metric:", w.Rule.Metric, "match:", w.Rule.Match, "error:", err.Error())
		return err
	}

	for _, tag := range w.Rule.Tags {
		fields := strings.SplitN(tag, "=", 2)
		reg, err := regexp.Compile(fields[1])
		if err != nil {
			log.Println("[ERROR] metric:", w.Rule.Metric, "tag:", tag, "error:", err.Error())
			return err
		}

		w.TagRegexp[fields[0]] = reg
	}

	log.Println("[INFO] metric:", w.Rule.Metric, "start success ")

	go w.Run()
	return nil
}

func (w *RuleWatcher) Stop() error {
	close(w.Exit)
	return nil
}

func (w *RuleWatcher) Notify(line *tail.Line) {

	select {
	case w.Lines <- line:
	default:
		log.Println("[ERROR] metric:", w.Rule.Metric, "full")
	}

}
