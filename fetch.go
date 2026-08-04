package main

import (
	"encoding/json"
	"fmt"
	"goFactServView/cwlog"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var FetchLock sync.Mutex
var spaceAgeCache = struct {
	sync.RWMutex
	servers map[uint64]bool
}{servers: make(map[uint64]bool)}

var fetchHTTPClient = func() *http.Client {
	return &http.Client{Timeout: ReqTimeout}
}

var spaceAgeMods = map[string]struct{}{
	"elevated-rails": {},
	"quality":        {},
	"recycler":       {},
	"space-age":      {},
}

type modInfo struct {
	Name    string
	Version string
}

type gameDetails struct {
	Mods []modInfo
}

func fetchServerList() error {

	//Don't refresh unless enough time has passed
	if time.Since(sParam.LastRefresh) < RefreshInterval {
		return nil
	}

	//Don't attempt if we attempted recently
	if time.Since(sParam.LastAttempt) < ReqThrottle {
		return nil
	}
	sParam.LastAttempt = time.Now().UTC()

	//Build query
	params := url.Values{}
	params.Add("username", *sParam.Username)
	params.Add("token", *sParam.Token)
	urlBuf := buildFetchURL(*sParam.URL, params)

	//HTTP GET
	req, err := http.NewRequest(http.MethodGet, urlBuf, nil)
	if err != nil {
		cwlog.DoLog(true, "fetchServerList: request build failed: %v", err)
		return err
	}

	req.Header.Set("User-Agent", UserAgent)

	//Get response
	res, getErr := fetchHTTPClient().Do(req)
	if getErr != nil {
		cwlog.DoLog(true, "fetchServerList: request failed: %v", getErr)
		return getErr
	}

	//Close once complete, if valid
	if res.Body != nil {
		defer res.Body.Close()
	}

	//Read all
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		cwlog.DoLog(true, "fetchServerList: read failed: %v", readErr)
		return readErr
	}

	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected upstream status %d: %s", res.StatusCode, shortenBody(body))
		cwlog.DoLog(true, "fetchServerList: %v", err)
		return err
	}

	//Unmarshal into temporary list
	newServerList := []ServerListItem{}
	jsonErr := json.Unmarshal(body, &newServerList)
	if jsonErr != nil {
		cwlog.DoLog(true, "fetchServerList: invalid JSON: %v", jsonErr)
		return jsonErr
	}

	//Remove Factorio tags
	for i, item := range newServerList {

		newServerList[i].Local.ConnectURL = MakeSteamURL(item.Host_address)
		newServerList[i].Name = RemoveFactorioTags(item.Name)
		newServerList[i].Description = RemoveFactorioTags(item.Description)
		for t, tag := range item.Tags {
			newServerList[i].Tags[t] = RemoveFactorioTags(tag)
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
		newServerList[i].Local.Players = len(item.Players)
		newServerList[i].Local.HasPlayers = len(item.Players) > 0
		mins := getMinutes(item)
		newServerList[i].Local.Minutes = getMinutes(item)
		newServerList[i].Local.TimeStr = updateTime(mins)
	}

	//Sort list
	newServerList = sortServers(false, newServerList, SORT_PLAYER)

	//Skip if result seems invalid/small
	if len(newServerList) <= MinValidCount {
		err := fmt.Errorf("upstream returned only %d servers", len(newServerList))
		cwlog.DoLog(true, "fetchServerList: %v", err)
		return err
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
	sParam.LastRefresh = time.Now().UTC()
	WriteServerCache()
	cwlog.DoLog(false, "Fetched server list at %v", time.Now())
	return nil
}

func buildFetchURL(baseURL string, params url.Values) string {
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		return baseURL + "/get-games?" + params.Encode()
	}
	return "https://" + baseURL + "/get-games?" + params.Encode()
}

func buildGameDetailsURL(baseURL string, gameID uint64) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	return baseURL + "/get-game-details/" + strconv.FormatUint(gameID, 10)
}

func requiresSpaceAge(mods []modInfo) bool {
	for _, mod := range mods {
		if _, found := spaceAgeMods[strings.ToLower(mod.Name)]; found {
			return true
		}
	}
	return false
}

// populateSpaceAgeRequirements looks up mod names only for the current page.
// The list endpoint exposes a mod count but not the names needed to distinguish
// vanilla Factorio from a game that requires the Space Age expansion.
func populateSpaceAgeRequirements(servers []ServerListItem, baseURL string) {
	var wg sync.WaitGroup
	client := fetchHTTPClient()
	for i := range servers {
		if servers[i].Game_id == 0 || parseVersion(servers[i].Application_version.Game_version).a < 2 {
			continue
		}

		gameID := servers[i].Game_id
		spaceAgeCache.RLock()
		requiresExpansion, found := spaceAgeCache.servers[gameID]
		spaceAgeCache.RUnlock()
		if found {
			servers[i].Local.SpaceAgeRequired = requiresExpansion
			continue
		}

		wg.Add(1)
		go func(server *ServerListItem) {
			defer wg.Done()

			req, err := http.NewRequest(http.MethodGet, buildGameDetailsURL(baseURL, server.Game_id), nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", UserAgent)

			res, err := client.Do(req)
			if err != nil {
				return
			}
			if res.Body == nil {
				return
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				return
			}

			var details gameDetails
			if err := json.NewDecoder(res.Body).Decode(&details); err != nil {
				return
			}

			requiresExpansion := requiresSpaceAge(details.Mods)
			server.Local.SpaceAgeRequired = requiresExpansion
			spaceAgeCache.Lock()
			spaceAgeCache.servers[server.Game_id] = requiresExpansion
			spaceAgeCache.Unlock()
		}(&servers[i])
	}
	wg.Wait()
}

func shortenBody(body []byte) string {
	const maxBody = 160

	text := strings.TrimSpace(string(body))
	if text == "" {
		return "<empty>"
	}
	if len(text) > maxBody {
		return text[:maxBody] + "..."
	}
	return text
}
