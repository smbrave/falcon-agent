package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/open-falcon/agent/cron"
	"github.com/open-falcon/agent/funcs"
	"github.com/open-falcon/agent/g"
	"github.com/open-falcon/agent/gather"
	"github.com/open-falcon/agent/http"
)

func main() {

	cfg := flag.String("c", "cfg.json", "configuration file")
	version := flag.Bool("v", false, "show version")
	check := flag.Bool("check", false, "check collector")

	gatherCfg := flag.String("gather", "gather.json", "gather configuration file")
	gatherCheck := flag.String("gather-check", "gather.txt", "gather test file")

	flag.Parse()

	if *version {
		fmt.Println(g.VERSION)
		os.Exit(0)
	}

	if *check {
		funcs.CheckCollector()
		os.Exit(0)
	}
	if *gatherCheck != "" {
		gather.Init(*gatherCfg)
		body, _ := ioutil.ReadFile(*gatherCheck)
		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			gather.Test(line)
		}
		os.Exit(0)
	}

	gather.Init(*gatherCfg)
	go gather.Run()

	g.ParseConfig(*cfg)

	g.InitRootDir()
	g.InitLocalIp()
	g.InitRpcClients()

	funcs.BuildMappers()

	go cron.InitDataHistory()

	cron.ReportAgentStatus()
	cron.SyncMinePlugins()
	cron.SyncBuiltinMetrics()
	cron.SyncTrustableIps()
	cron.Collect()

	go http.Start()

	select {}

}
