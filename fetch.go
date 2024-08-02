package main

import (
	"encoding/json"
	"goFactServView/cwlog"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var FetchLock sync.Mutex

func FetchServerList() {

	FetchLock.Lock()
	defer FetchLock.Unlock()

	if time.Since(sParam.LastRefresh) < RefreshInterval {
		return
	}

	if time.Since(sParam.LastAttempt) < ReqThrottle {
		return
	}

	sParam.LastAttempt = time.Now().UTC()

	hClient := http.Client{
		Timeout: ReqTimeout,
	}

	params := url.Values{}
	params.Add("username", *sParam.Username)
	params.Add("token", *sParam.Token)

	urlBuf := "https://" + *sParam.URL + "/get-games?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, urlBuf, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", UserAgent)

	res, getErr := hClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	tempServerList := []ServerListItem{}
	jsonErr := json.Unmarshal(body, &tempServerList)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	//Remove Factorio tags
	for i, item := range tempServerList {
		tempServerList[i].ConnectURL = MakeSteamURL(item.Host_address)
		tempServerList[i].Name = RemoveFactorioTags(item.Name)
		tempServerList[i].Description = RemoveFactorioTags(item.Description)
		for t, tag := range item.Tags {
			item.Tags[t] = RemoveFactorioTags(tag)
		}
		tempServerList[i].Modded = item.Mod_count > 0
		mins := getMinutes(item)
		tempServerList[i].Minutes = getMinutes(item)
		tempServerList[i].Time = updateTime(mins)
	}
	tempServerList = sortServers(tempServerList, SORT_PLAYER)

	sParam.LastRefresh = time.Now().UTC()
	cwlog.DoLog(false, "Fetched server list at %v", time.Now())

	if len(tempServerList) <= MinValidCount {
		return
	}
	sParam.ServerList.Servers = tempServerList
	sParam.ServersCount = len(tempServerList)
	WriteServerCache()
}
