package main

import (
	"bytes"
	"encoding/json"
	"goFactServView/cwlog"
	"os"
	"time"
)

func autoUpdateCert() {
	for {
		time.Sleep(time.Minute)

		filePath := "data/certs/fullchain.pem"
		initialStat, erra := os.Stat(filePath)

		if erra != nil {
			continue
		}

		for initialStat != nil {
			time.Sleep(time.Minute)

			stat, errb := os.Stat(filePath)
			if errb != nil {
				break
			}

			if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
				cwlog.DoLog(true, "Cert updated, closing.")
				time.Sleep(time.Second * 5)
				os.Exit(0)
				break
			}
		}

	}
}

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

			tempServerList := CacheData{}
			err := json.Unmarshal([]byte(file), &tempServerList)
			if err != nil {
				cwlog.DoLog(true, "ReadServerList: Unmarshal failure")
				return
			}
			if tempServerList.Version < CacheVersion {
				cwlog.DoLog(false, "Cache data is incompatable, skipping.")
				return
			}

			if len(tempServerList.Servers) > MinValidCount {
				sParam.ServerList.Servers = sortServers(tempServerList.Servers, SORT_PLAYER)
				sParam.LastRefresh = lastRefresh
				sParam.ServersCount = len(tempServerList.Servers)
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

	if len(sParam.ServerList.Servers) <= MinValidCount {
		return
	}

	sParam.ServerList.Version = CacheVersion
	if err := enc.Encode(sParam.ServerList); err != nil {
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
