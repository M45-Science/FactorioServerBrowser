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

func httpsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}

	if !strings.EqualFold(r.RequestURI, "/") && !strings.HasPrefix(r.RequestURI, "/?") {
		fileServer.ServeHTTP(w, r)
		return
	}

	FetchServerList()
	var tempParams *ServerStateData = &ServerStateData{
		URL:          sParam.URL,
		Query:        sParam.Query,
		Token:        sParam.Token,
		Username:     sParam.Username,
		LastRefresh:  sParam.LastRefresh,
		LastAttempt:  sParam.LastAttempt,
		UserAgent:    sParam.UserAgent,
		ServerList:   sParam.ServerList,
		ServersCount: sParam.ServersCount,
		ItemsPerPage: ItemsPerPage,
	}

	tempServersList := []ServerListItem{}
	page := 1

	cwlog.DoLog(false, "Request: %v", r.RequestURI)

	sortBy := SORT_PLAYER

	queryItems := r.URL.Query()
	if len(queryItems) > 0 {
		//cwlog.DoLog(false, "Query: %v", queryItems)
		found := false
		for key, values := range queryItems {
			if len(key) == 0 || len(values) == 0 {
				continue
			}

			if !found {
				if strings.EqualFold(key, "all") || values[0] == "" {
					tempServersList = tempParams.ServerList.Servers
					found = true
				} else if !found && strings.EqualFold(key, "name") {
					for s, server := range tempParams.ServerList.Servers {
						lName := strings.ToLower(server.Name)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lName, lVal) {
							tempServersList = append(tempServersList, tempParams.ServerList.Servers[s])
						}
					}
					found = true
				} else if !found && strings.EqualFold(key, "desc") {
					for s, server := range tempParams.ServerList.Servers {
						lDesc := strings.ToLower(server.Description)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lDesc, lVal) {
							tempServersList = append(tempServersList, tempParams.ServerList.Servers[s])
						}
					}
					found = true
				} else if !found && strings.EqualFold(key, "tag") {
					for s, server := range tempParams.ServerList.Servers {
						for _, tag := range server.Tags {
							if strings.EqualFold(values[0], tag) {
								tempServersList = append(tempServersList, tempParams.ServerList.Servers[s])
								break
							}
						}
					}
					found = true
				} else if !found && strings.EqualFold(key, "player") {
					for s, server := range tempParams.ServerList.Servers {
						for _, player := range server.Players {
							lPlayer := strings.ToLower(player)
							lVal := strings.ToLower(values[0])
							if strings.Contains(lPlayer, lVal) {
								tempServersList = append(tempServersList, tempParams.ServerList.Servers[s])
								break
							}
						}
					}
					found = true
				}
			}

			if strings.EqualFold(key, "sort-players") {
				sortBy = SORT_PLAYER
			} else if strings.EqualFold(key, "sort-name") {
				sortBy = SORT_NAME
			} else if strings.EqualFold(key, "sort-time") {
				sortBy = SORT_TIME
			} else if strings.EqualFold(key, "sort-rtime") {
				sortBy = SORT_RTIME
			} else if strings.EqualFold(key, "page") {
				val, err := strconv.ParseUint(values[0], 10, 64)
				if err != nil {
					continue
				} else {
					page = int(val)
				}
			}
		}
		if found {
			tempParams.ServerList.Servers = sortServers(tempServersList, sortBy)
			tempParams.ServersCount = len(tempServersList)
		}
	}
	paginateList(page, tempParams)
	err := tmpl.Execute(w, tempParams)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func paginateList(page int, tempParams *ServerStateData) {
	if page < 1 {
		page = 1
	}
	pageStart := (page - 1) * ItemsPerPage
	pageEnd := page * ItemsPerPage
	tempServerList := []ServerListItem{}

	if pageEnd > tempParams.ServersCount {
		pageEnd = tempParams.ServersCount
	}
	if pageStart > tempParams.ServersCount {
		pageStart = tempParams.ServersCount - tempParams.ItemsPerPage
	}
	if pageStart < 0 {
		return
	}

	for c := pageStart; c < pageEnd; c++ {
		tempParams.ServerList.Servers[c].Position = c + 1
		tempServerList = append(tempServerList, tempParams.ServerList.Servers[c])
	}
	tempParams.ServerList.Servers = tempServerList
	tempParams.NumPages = int(math.Ceil(float64(tempParams.ServersCount) / float64(tempParams.ItemsPerPage)))
	if page > tempParams.NumPages {
		tempParams.CurrentPage = tempParams.NumPages
	} else if page < 1 {
		tempParams.CurrentPage = 0
	} else {
		tempParams.CurrentPage = page
	}
}
