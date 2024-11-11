package main

import (
	"goFactServView/cwlog"
	"math"
	"net/http"
	"strconv"
	"strings"
)

const (
	SORT_PLAYER = iota
	SORT_NAME
	SORT_DESC
	SORT_TIME
	SORT_RTIME
)

// HTTP request handler
func reqHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}

	//If this isn't a query, pass to file server
	if !strings.EqualFold(r.RequestURI, "/") && !strings.HasPrefix(r.RequestURI, "/?") {
		fileServer.ServeHTTP(w, r)
		return
	}

	FetchLock.Lock()

	//If needed, refresh data
	fetchServerList()

	//Build temporary server params
	var tempParams *ServerStateData = &ServerStateData{
		URL:          sParam.URL,
		Query:        sParam.Query,
		Token:        sParam.Token,
		Username:     sParam.Username,
		LastRefresh:  sParam.LastRefresh,
		LastAttempt:  sParam.LastAttempt,
		UserAgent:    sParam.UserAgent,
		ServerList:   sParam.ServerList,
		ItemsPerPage: ItemsPerPage,
		VersionList:  sParam.VersionList,
		PlayerCount:  sParam.PlayerCount,
	}

	FetchLock.Unlock()

	//Create a blank server list
	page := 1

	//Log request
	cwlog.DoLog(false, "Request: %v", r.RequestURI)
	sortBy := SORT_PLAYER

	queryItems := r.URL.Query()
	if len(queryItems) > 0 {
		filterFound := false
		for key, values := range queryItems {

			//Skip if invalid
			if len(key) == 0 || len(values) == 0 {
				continue
			}

			if strings.EqualFold(key, "version") {
				tempParams.FVersion = values[0]
			}

			if strings.EqualFold(key, "vanilla") {
				tempParams.VanillaOnly = true
				tempParams.ModdedOnly = false
			} else if strings.EqualFold(key, "modded") {
				tempParams.VanillaOnly = false
				tempParams.ModdedOnly = true
			} else if strings.EqualFold(key, "both") {
				tempParams.VanillaOnly = false
				tempParams.ModdedOnly = false
			}

			if strings.EqualFold(key, "haspass") {
				tempParams.HasPass = true
			} else if strings.EqualFold(key, "anypass") {
				tempParams.AnyPass = true
			}

			if strings.EqualFold(key, "hasplay") {
				tempParams.HasPlay = true
			} else if strings.EqualFold(key, "noplay") {
				tempParams.NoPlay = true
			}

			//Don't parse multiple searches
			if !filterFound {
				if !filterFound && strings.EqualFold(key, "name") {
					var tempServers []ServerListItem
					filterFound = true
					tempParams.Searched = values[0]
					tempParams.SName = true
					if values[0] == "" {
						tempParams.ServerList.Servers = tempServers
						continue
					}
					for s, server := range tempParams.ServerList.Servers {
						lName := strings.ToLower(server.Name)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lName, lVal) {
							tempServers = append(tempServers, tempParams.ServerList.Servers[s])
							break
						}
					}
					tempParams.ServerList.Servers = tempServers
				} else if !filterFound && strings.EqualFold(key, "desc") {
					var tempServers []ServerListItem
					filterFound = true
					tempParams.Searched = values[0]
					tempParams.FDesc = true
					if values[0] == "" {
						tempParams.ServerList.Servers = tempServers
						continue
					}
					for s, server := range tempParams.ServerList.Servers {
						lDesc := strings.ToLower(server.Description)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lDesc, lVal) {
							tempServers = append(tempServers, tempParams.ServerList.Servers[s])
							break
						}
					}
					tempParams.ServerList.Servers = tempServers
				} else if !filterFound && strings.EqualFold(key, "tag") {
					var tempServers []ServerListItem
					filterFound = true
					tempParams.Searched = values[0]
					tempParams.FTag = true
					if values[0] == "" {
						tempParams.ServerList.Servers = tempServers
						continue
					}
					for s, server := range tempParams.ServerList.Servers {
						for _, tag := range server.Tags {
							if strings.EqualFold(values[0], tag) {
								tempServers = append(tempServers, tempParams.ServerList.Servers[s])
								break
							}
						}
					}
					tempParams.ServerList.Servers = tempServers
				} else if !filterFound && strings.EqualFold(key, "player") {
					var tempServers []ServerListItem
					filterFound = true
					tempParams.Searched = values[0]
					tempParams.FPlayer = true
					if values[0] == "" {
						tempParams.ServerList.Servers = tempServers
						continue
					}
					for s, server := range tempParams.ServerList.Servers {
						for _, player := range server.Players {
							lPlayer := strings.ToLower(player)
							lVal := strings.ToLower(values[0])
							if strings.Contains(lPlayer, lVal) {
								tempServers = append(tempServers, tempParams.ServerList.Servers[s])
								break
							}
						}
					}
					tempParams.ServerList.Servers = tempServers
				}
			}

			//Parse sorting arguments
			if strings.EqualFold(key, "sort-players") {
				sortBy = SORT_PLAYER
				tempParams.SPlayers = true
			} else if strings.EqualFold(key, "sort-name") {
				sortBy = SORT_NAME
				tempParams.SName = true
			} else if strings.EqualFold(key, "sort-time") {
				sortBy = SORT_TIME
				tempParams.STime = true
			} else if strings.EqualFold(key, "sort-rtime") {
				sortBy = SORT_RTIME
				tempParams.SRTime = true
			} else if strings.EqualFold(key, "page") {
				val, err := strconv.ParseUint(values[0], 10, 64)
				if err != nil {
					continue
				} else {
					page = int(val)
				}
			}
		}
	}
	filterServers(tempParams)
	tempParams.ServerList.Servers = sortServers(tempParams.ServerList.Servers, sortBy)
	//Build a single page of results

	paginateList(page, tempParams)

	//Execute template
	err := tmpl.Execute(w, tempParams)
	if err != nil {
		cwlog.DoLog(true, "Error: %v", err)
	}
}

func filterServers(tempParams *ServerStateData) {
	var tempServers []ServerListItem
	for _, server := range tempParams.ServerList.Servers {
		if tempParams.ModdedOnly && !server.Local.Modded {
			continue
		}
		if tempParams.VanillaOnly && server.Local.Modded {
			continue
		}
		if !tempParams.AnyPass {
			if tempParams.HasPass && !server.Has_password {
				continue
			}
			if !tempParams.HasPass && server.Has_password {
				continue
			}
		}
		if tempParams.HasPlay && !server.Local.HasPlayers {
			continue
		}
		if tempParams.NoPlay && server.Local.HasPlayers {
			continue
		}
		if tempParams.FVersion != "" &&
			server.Application_version.Game_version != tempParams.FVersion {
			continue
		}
		tempServers = append(tempServers, server)
	}
	tempParams.ServerList.Servers = tempServers
	tempParams.ServersCount = len(tempParams.ServerList.Servers)
}

// Present a single page of results
func paginateList(page int, tempParams *ServerStateData) {
	if page < 1 {
		page = 1
	}
	//Calculate list position
	pageStart := (page - 1) * ItemsPerPage
	pageEnd := page * ItemsPerPage

	//Build temp list
	tempServerList := []ServerListItem{}

	//Calculate start/end of page
	if pageEnd > tempParams.ServersCount {
		pageEnd = tempParams.ServersCount
	}
	if pageStart > tempParams.ServersCount {
		pageStart = tempParams.ServersCount - tempParams.ItemsPerPage
	}
	//Reject invalid page
	if pageStart < 0 {
		cwlog.DoLog(true, "Page start less than 0.")
		return
	}

	//Put results into temp list
	for c := pageStart; c < pageEnd; c++ {
		tempServerList = append(tempServerList, tempParams.ServerList.Servers[c])
	}

	//Put results into page
	tempParams.ServerList.Servers = tempServerList
	tempParams.NumPages = int(math.Ceil(float64(tempParams.ServersCount) / float64(tempParams.ItemsPerPage)))

	//Handle invalid or nil page numbers
	if page > tempParams.NumPages {
		tempParams.CurrentPage = tempParams.NumPages
	} else if page < 1 {
		tempParams.CurrentPage = 0
	} else {
		tempParams.CurrentPage = page
	}
}
