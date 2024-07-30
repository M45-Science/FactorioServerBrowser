package main

import (
	"bytes"
	"encoding/json"
	"os"
	"time"
)

func ReadServerList(ServState *ServerStateData) {

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
				errLog("ReadServerList: Unmarshal failure")
				return
			}

			if len(tempServerList) > MinValidCount {
				ServState.ServersList = sortServers(tempServerList)
				ServState.LastRefresh = lastRefresh
				ServState.ServersCount = len(tempServerList)
				errLog("Read cached server list.")
			}
			return
		} else {
			errLog("ReadServerList: Read file failure")
			return
		}
	}
}

func WriteServerList(ServState *ServerStateData) {

	tempPath := CacheFile + ".tmp"

	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	if len(ServState.ServersList) <= MinValidCount {
		return
	}

	if err := enc.Encode(ServState.ServersList); err != nil {
		errLog("WriteServerList: enc.Encode failure")
		return
	}

	_, err := os.Create(tempPath)

	if err != nil {
		errLog("WriteServerList: os.Create failure")
		return
	}

	err = os.WriteFile(tempPath, outbuf.Bytes(), 0644)

	if err != nil {
		errLog("WriteServerList: Write file failure")
	}

	err = os.Rename(tempPath, CacheFile)

	if err != nil {
		errLog("Couldn't rename cache file.")
		return
	}
}
