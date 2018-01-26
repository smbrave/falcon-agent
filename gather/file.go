package gather

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"bytes"
	"errors"
	"regexp"

	"strconv"

	"github.com/open-falcon/agent/g"
	"github.com/open-falcon/common/model"
)

func NewGatherWorker(gf *GatherFile) *GatherWorker {
	return &GatherWorker{
		GFile:   gf,
		ReadPos: 0,
	}
}

//
func (gw *GatherWorker) ParseLine(line *string, regexpLine *regexp.Regexp, regexpTags map[string]*regexp.Regexp, item *GatherItem) (float64, string, error) {
	value := float64(0)
	tag_key := ""

	lineMathch := regexpLine.FindStringSubmatch(*line)
	if len(lineMathch) == 0 {
		return 0, "", fmt.Errorf("not match :%s", item.Rule)
	}

	if len(lineMathch) >= 2 {
		value, _ = strconv.ParseFloat(lineMathch[1], 64)
	}
	tagsResult := make([]string, 0)
	for k, v := range regexpTags {
		tagMatch := v.FindStringSubmatch(*line)
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

//上报数据
func (gw *GatherWorker) ReportData(item *GatherItem, tagsStat map[string]*GatherStat) {
	if len(tagsStat) == 0 {
		return
	}
	tp := strings.ToUpper(item.Type)
	hostname, _ := os.Hostname()
	mertics := make([]*model.MetricValue, 0)
	for k, v := range tagsStat {
		metric := new(model.MetricValue)
		metric.Metric = item.Metric
		switch tp {
		case "GAUGE":
			metric.Value = v.Counter
		case "SUM":
			metric.Value = v.Total
		case "MAX":
			metric.Value = v.Max
		case "MIN":
			metric.Value = v.Min
		case "AVG":
			metric.Value = v.Total / v.Counter
		}

		metric.Tags = k
		metric.Step = int64(gw.GFile.ReportStep)
		metric.Timestamp = time.Now().Unix()
		metric.Endpoint = hostname
		metric.Type = "GAUGE"
		mertics = append(mertics, metric)
	}

	var resp model.TransferResponse
	g.SendMetrics(mertics, &resp)
	log.Printf("=> metric:%s resp:%s\n", mertics[0].Metric, len(mertics), resp.String())
	for i, me := range mertics {
		log.Printf("==> idx:%d mertic:%v \n", i, me)
	}
}

//搜集数据
func (gw *GatherWorker) GatherData(tag_key string, value float64, tagsStat map[string]*GatherStat) {
	var stat *GatherStat = nil
	var ok bool

	//有tag的汇总一份总数据
	if tag_key != "" {
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
	if stat, ok = tagsStat[tag_key]; !ok {
		stat = new(GatherStat)
		stat.Rest()
		tagsStat[tag_key] = stat
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

//单指标上报
func (gw *GatherWorker) SubWorker(ch chan *LogInfo, item *GatherItem) {
	//标签正则表达
	tagRegexpMap := make(map[string]*regexp.Regexp)
	lineRegexpMap, _ := regexp.Compile(item.Rule)
	for _, tag := range item.Tags {
		fields := strings.SplitN(tag, "=", 2)
		reg, _ := regexp.Compile(fields[1])
		tagRegexpMap[fields[0]] = reg
	}

	tagsStat := make(map[string]*GatherStat)

	ticker := time.NewTicker(time.Second * time.Duration(gw.GFile.ReportStep))
	defer ticker.Stop()
	for {
		select {
		//数据采集
		case logInfo := <-ch:
			value, tag_key, err := gw.ParseLine(logInfo.LogLine, lineRegexpMap, tagRegexpMap, item)
			if err != nil {
				//log.Println("line:", logInfo.LogLine, "error:", err.Error())
				continue
			}
			gw.GatherData(tag_key, value, tagsStat)

			//定时上报
		case <-ticker.C:
			gw.ReportData(item, tagsStat)
			tagsStat = make(map[string]*GatherStat)
		}

	}
}

//一个文件解析
func (gw *GatherWorker) Worker() {

	gw.DataChan = make([]chan *LogInfo, 0)

	//每个采集指标一个协程独立任务
	for i := 0; i < len(gw.GFile.Items); i++ {
		ch := make(chan *LogInfo, 100)
		gw.DataChan = append(gw.DataChan, ch)
		go gw.SubWorker(ch, &gw.GFile.Items[i])
	}

	buf := make([]byte, 1024*1024)
	ticker := time.NewTicker(time.Second * time.Duration(gw.GFile.GatherStep))
	defer ticker.Stop()

	for _ = range ticker.C {
		//打开文件
		fd, err := os.Open(gw.GFile.File)
		if err != nil {
			log.Println("open file:", gw.GFile.File, "error:", err.Error())
			continue
		}

		//获取文件统计信息
		fileInfo, err := fd.Stat()
		if err != nil {
			log.Println("file:", gw.GFile.File, "stat error:", err.Error())
			fd.Close()
			continue
		}

		//没有新增内容
		fileSzie := fileInfo.Size()
		if gw.ReadPos == fileSzie {
			fd.Close()
			continue
		}

		//文件有滚动或重写
		if gw.ReadPos > fileSzie {
			log.Println("file:", gw.GFile.File, "size:", fileSzie, "read:", gw.ReadPos)
			gw.ReadPos = 0
			fd.Close()
			continue
		}

		//读完当前更新的所有数据
		for {
			readLen, err := fd.ReadAt(buf, gw.ReadPos)
			if err != nil && err != io.EOF {
				log.Println("file:", gw.GFile.File, "read at error:", err.Error())
				break
			}
			if readLen == 0 {
				break
			}

			readLen = bytes.LastIndex(buf[0:readLen], []byte("\n"))
			if readLen == -1 {
				break
			}

			//读取到的数据
			lines := strings.Split(string(buf[0:readLen]), "\n")
			for i, line := range lines {
				if len(line) == 0 {
					continue
				}

				//获取日志的时间
				logTime, err := gw.GetLineTime(&line)
				if err != nil {
					log.Println("line:", line, "time format error:", err.Error())
					continue
				}

				//丢弃一个上报周期之前的日志
				if logTime.Add(time.Second * time.Duration(gw.GFile.ReportStep)).Before(time.Now()) {
					continue
				}

				logInfo := new(LogInfo)
				logInfo.LogLine = &lines[i]
				logInfo.LogTime = logTime

				//发送给每个采集子任务
				for _, ch := range gw.DataChan {
					ch <- logInfo
				}
			}

			//修改读取偏移
			gw.ReadPos += int64(readLen)
			if gw.ReadPos >= fileSzie {
				break
			}
		}
		fd.Close()

	}
}

//获取日志时间
func (gw *GatherWorker) GetLineTime(line *string) (*time.Time, error) {

	timeStr := strings.TrimLeft(*line, "\r\n\t ")
	if len(timeStr) < len(gw.GFile.Format) {
		return nil, errors.New("time len error")
	}

	t, err := time.ParseInLocation(gw.GFile.Format, timeStr[0:len(gw.GFile.Format)], localtion)
	return &t, err
}

//启动所有采集任务
func Run() {
	if config.Enable == DISABLE {
		return
	}

	for i, gf := range config.Files {
		if gf.Enable == DISABLE {
			continue
		}
		fmt.Println(fmt.Sprintf("gather_file:%+v", gf))
		gw := NewGatherWorker(&config.Files[i])
		go gw.Worker()
	}
}
