package main

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

// Log errors, sprintf format
func errLog(format string, args ...any) {
	_, filePath, line, _ := runtime.Caller(1)
	file := path.Base(filePath)
	data := fmt.Sprintf(format, args...)
	buf := fmt.Sprintf("%v:%v: %v", file, line, data)

	fmt.Println(buf)
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
