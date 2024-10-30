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

func fetchServerList() {

	//Don't refresh unless enough time has passed
	if time.Since(sParam.LastRefresh) < RefreshInterval {
		return
	}

	//Don't attempt if we attempted recently
	if time.Since(sParam.LastAttempt) < ReqThrottle {
		return
	}
	sParam.LastAttempt = time.Now().UTC()

	//Set timeout
	hClient := http.Client{
		Timeout: ReqTimeout,
	}

	//Build query
	params := url.Values{}
	params.Add("username", *sParam.Username)
	params.Add("token", *sParam.Token)
	urlBuf := "https://" + *sParam.URL + "/get-games?" + params.Encode()

	//HTTP GET
	req, err := http.NewRequest(http.MethodGet, urlBuf, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", UserAgent)

	//Get response
	res, getErr := hClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	//Close once complete, if valid
	if res.Body != nil {
		defer res.Body.Close()
	}

	//Read all
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	//Unmarshal into temporary list
	newServerList := []ServerListItem{}
	jsonErr := json.Unmarshal(body, &newServerList)
	if jsonErr != nil {
		log.Fatal("fetch server list: " + jsonErr.Error() + "\n" + string(body))
	}

	//Remove Factorio tags
	for i, item := range newServerList {

		newServerList[i].Local.ConnectURL = MakeSteamURL(item.Host_address)
		newServerList[i].Name = RemoveFactorioTags(item.Name)
		newServerList[i].Description = RemoveFactorioTags(item.Description)
		for t, tag := range item.Tags {
			item.Tags[t] = RemoveFactorioTags(tag)
		}

		//If name is only tags, allow it.
		if newServerList[i].Name == "" {
			newServerList[i].Name = item.Name
		}
		//If server name is still nothing, put something into that field.
		if newServerList[i].Name == "" {
			newServerList[i].Name = "Unnamed Server"
		}
		//Convert some of the data for web
		newServerList[i].Local.Modded = item.Mod_count > 0
		mins := getMinutes(item)
		newServerList[i].Local.Minutes = getMinutes(item)
		newServerList[i].Local.TimeStr = updateTime(mins)
	}

	//Sort list
	newServerList = sortServers(newServerList, SORT_PLAYER)

	//Save last refresh time
	sParam.LastRefresh = time.Now().UTC()
	cwlog.DoLog(false, "Fetched server list at %v", time.Now())

	//Skip if result seems invalid/small
	if len(newServerList) <= MinValidCount {
		return
	}

	//Apply temporary list to global list
	sParam.ServerList.Servers = newServerList
	sParam.ServersCount = len(sParam.ServerList.Servers)
	getVersions()

	totalPlayers := 0
	for _, item := range newServerList {
		totalPlayers = totalPlayers + len(item.Players)
	}
	sParam.PlayerCount = totalPlayers

	//Write to cache file
	WriteServerCache()
}
