package gatherv2

import (
	"encoding/json"
	"log"
	"time"

	"github.com/open-falcon/agent/g"
	"github.com/toolkits/net/httplib"
)

var (
	fileWatcher []*FileWatcher
	lastVersion int64
)

func Monitor(url string) *AgentData {
	resp, err := httplib.Get(url).Header("ak", "58a72222d1b7e5ab5a2b3c95a0dda245").Bytes()
	if err != nil {
		log.Println("[ERROR] url:", url, "error:", err.Error())
		return nil
	}

	var rsp ConfigResponse
	if err := json.Unmarshal(resp, &rsp); err != nil {
		log.Println("[ERROR] url:", url, "body:", string(resp), "error:", err.Error())
		return nil
	}
	if rsp.Errno != 0 {
		log.Println("[ERROR] url:", url, "body:", string(resp), "errno:", rsp.Errno, "message:", rsp.Errmsg)
		return nil
	}

	if rsp.Data.Update == lastVersion {
		return nil
	}
	lastVersion = rsp.Data.Update
	return rsp.Data
}

func Update(data *AgentData) {

	//如果有任务在运行先停止任务
	if fileWatcher != nil && len(fileWatcher) != 0 {
		for _, fw := range fileWatcher {
			if err := fw.Stop(); err != nil {
				log.Println("[ERROR] file:", fw.MFile.File, "stop error:", err.Error())
				continue
			}
		}
	}

	//创建新任务
	fileWatcher = make([]*FileWatcher, 0)
	for _, file := range data.Files {
		fw := NewFileWatcher(file)
		if err := fw.Start(); err != nil {
			log.Println("[ERROR] file:", ObjectString(file), "error:", err.Error())
			continue
		}
		fileWatcher = append(fileWatcher, fw)
	}

}

func Run() {
	ticker := time.NewTicker(10 * time.Second)
	hostname, _ := g.Hostname()
	falcon := g.Config().Falcon
	if falcon == "" {
		return
	}
	url := g.Config().Falcon + "?hostname=" + hostname
	data := Monitor(url)
	if data != nil {
		log.Println("[data]", ObjectString(data))
		Update(data)
	}

	for {
		select {
		case <-ticker.C:
			data := Monitor(url)
			if data != nil {
				log.Println("[data]", ObjectString(data))
				Update(data)
			}
		}
	}
}
