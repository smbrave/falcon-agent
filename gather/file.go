package gather

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/open-falcon/agent/g"
	"github.com/open-falcon/common/model"
)

type GatherWorker struct {
	GFile      *GatherFile
	ReadPos    int64
	DataChan   []chan string
	MetricChan chan *model.MetricValue
}

func NewGatherWorker(gf *GatherFile) *GatherWorker {
	return &GatherWorker{
		GFile:   gf,
		ReadPos: 0,
	}
}

//单指标上报
func (gw *GatherWorker) SubWorker(ch chan string, item *GatherItem) {
	ticker := time.NewTicker(time.Second * time.Duration(gw.GFile.ReportStep))

	var value int = 0
	hostname, _ := os.Hostname()
	for {
		select {
		case <-ch:
			value += 1

		case <-ticker.C:
			metric := new(model.MetricValue)
			metric.Metric = item.Metric
			metric.Value = value
			metric.Step = int64(gw.GFile.ReportStep)
			metric.Timestamp = time.Now().Unix()
			metric.Endpoint = hostname
			metric.Type = "GAUGE"

			mertics := make([]*model.MetricValue, 0)
			mertics = append(mertics, metric)
			value = 0
			var resp model.TransferResponse
			g.SendMetrics(mertics, &resp)
			log.Println("<====", resp)
			mertics = make([]*model.MetricValue, 0)
		}

	}
}

func (gw *GatherWorker) Report() {
	ticker := time.NewTicker(time.Second * time.Duration(gw.GFile.ReportStep))
	mertics := make([]*model.MetricValue, 0)
	for {
		select {
		case mertic := <-gw.MetricChan:
			mertics = append(mertics, mertic)
		case <-ticker.C:
			var resp model.TransferResponse
			g.SendMetrics(mertics, &resp)
			log.Println("<====", resp)
			mertics = make([]*model.MetricValue, 0)
		}
	}

}

//一个文件解析
func (gw *GatherWorker) Worker() {

	gw.DataChan = make([]chan string, 0)
	gw.MetricChan = make(chan *model.MetricValue, 1024)

	for i := 0; i < len(gw.GFile.Items); i++ {
		ch := make(chan string, 100)
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
			continue
		}

		//log.Println("file:", fileInfo.Name(), "size:", fileInfo.Size(), "read pos:", gw.ReadPos)
		//没有新增内容
		fileSzie := fileInfo.Size()
		if gw.ReadPos == fileSzie {
			continue
		}

		//文件有滚动或重写
		if gw.ReadPos > fileSzie {
			log.Println("file:", gw.GFile.File, "size:", fileSzie, "read:", gw.ReadPos)
			gw.ReadPos = 0
			continue
		}

		//读完当前更新的所有数据
		for {
			readLen, err := fd.ReadAt(buf, gw.ReadPos)
			if err != nil && err != io.EOF {
				log.Println("file:", gw.GFile.File, "read at error:", err.Error())
				break
			}

			if readLen == 0 && err == io.EOF {
				break
			}

			lines := strings.Split(string(buf[0:readLen]), "\n")
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}

				for _, ch := range gw.DataChan {
					ch <- line
				}
			}

			gw.ReadPos += int64(readLen)
			if gw.ReadPos >= fileSzie {
				break
			}
		}

	}
}

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
