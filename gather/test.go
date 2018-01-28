package gather

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

func Test(line string) {

	for _, file := range config.Files {

		for _, item := range file.Items {
			lineRegexpMap, err := regexp.Compile(item.Rule)
			if err != nil {
				log.Println("metric:", item.Metric, "rule:", item.Rule, "regexp err:", err.Error())
				continue
			}

			lineMathch := lineRegexpMap.FindStringSubmatch(line)
			if len(lineMathch) == 0 {
				//log.Println("no match line:", line, "rule:", item.Rule)
				continue
			}

			var value float64
			if len(lineMathch) >= 2 {
				value, _ = strconv.ParseFloat(lineMathch[1], 64)
			}
			if len(item.Tags) == 0 {
				log.Printf("metric:%s value:%.6f\n",
					item.Metric, value)
			}
			for _, tag := range item.Tags {
				fields := strings.SplitN(tag, "=", 2)
				reg, err := regexp.Compile(fields[1])
				if err != nil {
					log.Println("metric:", item.Metric, "tag:", fields[1], "regexp err:", err.Error())
					continue
				}
				tagMatch := reg.FindStringSubmatch(line)
				if len(lineMathch) == 0 {
					continue
				}
				if len(tagMatch) >= 2 {
					log.Printf("metric:%s value:%.6f tag:%s\n",
						item.Metric, value, tagMatch[1])
				}
			}

		}
	}
}
