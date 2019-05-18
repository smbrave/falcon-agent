package gatherv2

import (
	"fmt"
	"os"
	"time"

	"github.com/hpcloud/tail"
)

type FileWatcher struct {
	MFile    *FileConf
	Observer []IObserver
	Tail     *tail.Tail
}

func NewFileWatcher(file *FileConf) *FileWatcher {
	return &FileWatcher{
		MFile:    file,
		Observer: make([]IObserver, 0),
	}
}

func (w *FileWatcher) Run() {

	for line := range w.Tail.Lines {
		//发送给每个观察则
		for _, ob := range w.Observer {
			ob.Notify(line)
		}
	}
	fmt.Println("[INFO] file:", w.MFile.File, "stoped")
}

func (w *FileWatcher) Start() error {
	//检查文件
	fileInfo, err := os.Stat(w.MFile.File)
	if err != nil {
		fmt.Println("[ERROR] os.Stat(w.MFile.File) file:", w.MFile.File, "error:", err.Error())
		return err
	}

	//创建子观察者
	for _, rule := range w.MFile.Rules {
		rule := NewRuleWatcher(w, rule)
		if err := rule.Start(); err != nil {
			fmt.Println("[ERROR] rule run error:", err.Error())
			continue
		}
		w.Observer = append(w.Observer, rule)
	}

	//创建tail任务
	var tailConfig tail.Config
	tailConfig.Follow = true
	tailConfig.ReOpen = true

	t, err := tail.TailFile(w.MFile.File, tailConfig)
	if err != nil {
		fmt.Println("[ERROR] file:", w.MFile.File, "error:", err.Error())
		time.Sleep(time.Second)
		return err
	}

	//设置初始化偏移
	t.Location = new(tail.SeekInfo)
	t.Location.Offset = fileInfo.Size()
	fmt.Println("[INFO] file:", w.MFile.File, "start success! offset:", fileInfo.Size())
	w.Tail = t
	go w.Run()
	return nil
}

func (w *FileWatcher) Stop() error {

	for _, ob := range w.Observer {
		if err := ob.Stop(); err != nil {
			fmt.Println("[ERROR] stop ", w.MFile.File, "rule error:", err.Error())
		}
	}
	return w.Tail.Stop()
}
