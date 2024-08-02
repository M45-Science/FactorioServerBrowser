package main

import (
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hako/durafmt"
)

func bgUpdateList() {
	for {
		time.Sleep(BGFetchInterval)
		FetchServerList()
	}
}

func parseTemplate() {
	var err error
	tmpl, err = template.ParseFiles("data/template.html")
	if err != nil {
		panic(err)
	}
}

var tUnits durafmt.Units

func setupDurafmt() {
	var err error
	tUnits, err = durafmt.DefaultUnitsCoder.Decode("yr:yrs,wk:wks,day:days,hr:hrs,min:mins,sec:secs,ms:ms,μs:μs")
	if err != nil {
		panic(err)
	}
}

func sortServers(list []ServerListItem, sortBy int) []ServerListItem {
	if sortBy == SORT_NAME {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})
	} else if sortBy == SORT_TIME {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Minutes < list[j].Minutes
		})
	} else if sortBy == SORT_RTIME {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Minutes > list[j].Minutes
		})
	} else {
		sort.Slice(list, func(i, j int) bool {
			iNum := len(list[i].Players)
			jNum := len(list[j].Players)
			if iNum == jNum {
				return list[i].Name < list[j].Name
			}
			return iNum > jNum
		})
	}
	return list
}

func updateTime(mins int) string {
	if mins == 0 {
		return "0 min"
	}
	return durafmt.Parse(time.Duration(mins) * time.Minute).LimitFirstN(2).Format(tUnits)
}

func getMinutes(item ServerListItem) int {
	played, err := time.ParseDuration(fmt.Sprintf("%vm", item.Game_time_elapsed))
	if err == nil {
		return int(played.Minutes())
	}

	return 0
}

var (
	remFactTag      *regexp.Regexp = regexp.MustCompile(`\[/[^][]+\]`)
	remFactCloseTag *regexp.Regexp = regexp.MustCompile(`\[(.*?)=(.*?)\]`)
)

func RemoveFactorioTags(input string) string {
	buf := input
	buf = remFactCloseTag.ReplaceAllString(buf, "")
	buf = remFactTag.ReplaceAllString(buf, "")

	buf = strings.Replace(buf, "\n\r", "\n", -1)
	buf = strings.Replace(buf, "\r", "\n", -1)
	buf = strings.Replace(buf, "\n\n", "\n", -1)
	buf = strings.Replace(buf, "\n", "  ", -1)

	return buf
}

func MakeSteamURL(host string) string {
	buf := fmt.Sprintf("https://go-game.net/gosteam/427520.--mp-connect%%20%v", host)
	return buf
}
