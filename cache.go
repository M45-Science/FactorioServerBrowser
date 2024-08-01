package main

import (
	"bytes"
	"encoding/json"
	"goFactServView/cwlog"
	"os"
	"time"
)

func ReadServerList() {

	_, err := os.Stat(CacheFile)
	notfound := os.IsNotExist(err)

	if notfound {
		return
	} else { /* Otherwise just read in the config */
		info, err := os.Stat(CacheFile)
		lastRefresh := time.Time{}
		if err == nil {
			lastRefresh = info.ModTime()
		}

		file, err := os.ReadFile(CacheFile)

		if file != nil && err == nil {

			tempServerList := []ServerListItem{}
			err := json.Unmarshal([]byte(file), &tempServerList)
			if err != nil {
				cwlog.DoLog(true, "ReadServerList: Unmarshal failure")
				return
			}

			if len(tempServerList) > MinValidCount {
				sParam.ServersList = sortServers(tempServerList)
				sParam.LastRefresh = lastRefresh
				sParam.ServersCount = len(tempServerList)
				cwlog.DoLog(true, "Read cached server list.")
			}
			return
		} else {
			cwlog.DoLog(true, "ReadServerList: Read file failure")
			return
		}
	}
}

func WriteServerList() {

	tempPath := CacheFile + ".tmp"

	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	if len(sParam.ServersList) <= MinValidCount {
		return
	}

	if err := enc.Encode(sParam.ServersList); err != nil {
		cwlog.DoLog(true, "WriteServerList: enc.Encode failure")
		return
	}

	_, err := os.Create(tempPath)

	if err != nil {
		cwlog.DoLog(true, "WriteServerList: os.Create failure")
		return
	}

	err = os.WriteFile(tempPath, outbuf.Bytes(), 0644)

	if err != nil {
		cwlog.DoLog(true, "WriteServerList: Write file failure")
	}

	err = os.Rename(tempPath, CacheFile)

	if err != nil {
		cwlog.DoLog(true, "Couldn't rename cache file.")
		return
	}
}
