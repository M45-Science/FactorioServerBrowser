package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/hako/durafmt"
)

func updateTime(item ServerListItem) string {
	output := "unknown"

	played, err := time.ParseDuration(fmt.Sprintf("%vm", item.Game_time_elapsed))
	if err == nil {
		played = played.Round(time.Second)
		output = durafmt.Parse(played).LimitFirstN(2).Format(tUnits)
	}

	return output
}

func RemoveFactorioTags(input string) string {
	buf := input
	buf = regb.ReplaceAllString(buf, "")
	buf = rega.ReplaceAllString(buf, "")

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
