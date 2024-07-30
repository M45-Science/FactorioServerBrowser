package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	Version   = "0.1.1"
	VDate     = "07302024-0323"
	ProgName  = "goFactServView"
	UserAgent = ProgName + "-" + Version
	VString   = ProgName + "v" + Version + " (" + VDate + ") "
	CacheFile = "cache.json"

	ReqTimeout      = time.Second * 5
	ReqThrottle     = time.Second * 15
	RefreshInterval = time.Minute * 5

	BGFetchInterval = time.Hour * 3
	//If we get less results than this, assume the data is incomplete or corrupt
	MinValidCount = 25
	ItemsPerPage  = 25
)

var (
	rega *regexp.Regexp = regexp.MustCompile(`\[/[^][]+\]`)
	regb *regexp.Regexp = regexp.MustCompile(`\[(.*?)=(.*?)\]`)
)

func main() {
	var sParam *ServerStateData = &ServerStateData{}
	sParam.URL = flag.String("url", "multiplayer.factorio.com", "domain name to query")
	sParam.Token = flag.String("token", "", "Matchmaking API token")
	sParam.Username = flag.String("username", "", "Matchmaking API username")
	sParam.NoFetch = flag.Bool("noFetch", false, "Never fetch the server list, for testing only.")
	sParam.UserAgent = UserAgent
	bindIP := flag.String("ip", "localhost", "IP to bind to")
	bindPort := flag.Int("port", 8080, "port to bind to for HTTP")
	getVersion := flag.Bool("version", false, "Get program version")
	flag.Parse()

	server := &http.Server{}
	server.Addr = fmt.Sprintf("%v:%v", *bindIP, *bindPort)

	if *getVersion {
		fmt.Println(VString)
		return
	}

	if *sParam.Token == "" || *sParam.Username == "" {
		errLog("You must supply a username and token. -h for help.")
		os.Exit(1)
		return
	}

	ReadServerList(sParam)

	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.RequestURI, "/") && !strings.HasPrefix(r.RequestURI, "/?") {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		sParam.FetchServerList()
		var tempParams *ServerStateData = &ServerStateData{
			URL:          sParam.URL,
			Query:        sParam.Query,
			Token:        sParam.Token,
			Username:     sParam.Username,
			LastRefresh:  sParam.LastRefresh,
			LastAttempt:  sParam.LastAttempt,
			UserAgent:    sParam.UserAgent,
			NoFetch:      sParam.NoFetch,
			ServersList:  sParam.ServersList,
			ServersCount: sParam.ServersCount,
			ItemsPerPage: ItemsPerPage,
		}

		tempServersList := []ServerListItem{}
		page := 1

		//errLog("Request: %v", r.RequestURI)

		queryItems := r.URL.Query()
		if len(queryItems) > 0 {
			//errLog("Query: %v", queryItems)
			found := false
			for key, values := range queryItems {
				if len(key) == 0 || len(values) == 0 {
					continue
				}
				if values[0] == "" {
					continue
				}
				if strings.EqualFold(key, "page") {
					val, err := strconv.ParseUint(values[0], 10, 64)
					if err != nil {
						continue
					} else {
						page = int(val)
					}
				} else if strings.EqualFold(key, "name") {
					for s, server := range tempParams.ServersList {
						lName := strings.ToLower(server.Name)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lName, lVal) {
							tempServersList = append(tempServersList, tempParams.ServersList[s])
						}
					}
					found = true
					break
				} else if strings.EqualFold(key, "desc") {
					for s, server := range tempParams.ServersList {
						lDesc := strings.ToLower(server.Description)
						lVal := strings.ToLower(values[0])
						if strings.Contains(lDesc, lVal) {
							tempServersList = append(tempServersList, tempParams.ServersList[s])
						}
					}
					found = true
					break
				} else if strings.EqualFold(key, "tag") {
					for s, server := range tempParams.ServersList {
						for _, tag := range server.Tags {
							if strings.EqualFold(values[0], tag) {
								tempServersList = append(tempServersList, tempParams.ServersList[s])
								break
							}
						}
					}
					found = true
					break
				} else if strings.EqualFold(key, "player") {
					for s, server := range tempParams.ServersList {
						for _, player := range server.Players {
							lPlayer := strings.ToLower(player)
							lVal := strings.ToLower(values[0])
							if strings.Contains(lPlayer, lVal) {
								tempServersList = append(tempServersList, tempParams.ServersList[s])
								break
							}
						}
					}
					found = true
					break
				}
			}
			if found {
				tempParams.ServersList = sortServers(tempServersList)
				tempParams.ServersCount = len(tempServersList)
			}
		}
		paginateList(page, tempParams)
		err := tmpl.Execute(w, tempParams)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Start the server
	server.ListenAndServe()

	//Refresh cache infrequently
	go func(sp *ServerStateData) {
		for {
			time.Sleep(BGFetchInterval)
			sp.FetchServerList()
		}
	}(sParam)

	signalHandle := make(chan os.Signal, 1)
	signal.Notify(signalHandle, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-signalHandle
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
		tempParams.ServersList[c].Position = c + 1
		tempServerList = append(tempServerList, tempParams.ServersList[c])
	}
	tempParams.ServersList = tempServerList
	tempParams.CurrentPage = page
	tempParams.NumPages = int(math.Ceil(float64(tempParams.ServersCount)/float64(tempParams.ItemsPerPage)) + 1)
}

var FetchLock sync.Mutex

func (ServData *ServerStateData) FetchServerList() {

	if *ServData.NoFetch {
		return
	}

	FetchLock.Lock()
	defer FetchLock.Unlock()

	if time.Since(ServData.LastRefresh) < RefreshInterval {
		return
	}

	if time.Since(ServData.LastAttempt) < ReqThrottle {
		return
	}

	ServData.LastAttempt = time.Now().UTC()

	hClient := http.Client{
		Timeout: ReqTimeout,
	}

	params := url.Values{}
	params.Add("username", *ServData.Username)
	params.Add("token", *ServData.Token)

	urlBuf := "https://" + *ServData.URL + "/get-games?" + params.Encode()

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
	}
	tempServerList = sortServers(tempServerList)

	ServData.LastRefresh = time.Now().UTC()
	errLog("Fetched server list at %v", time.Now())

	if len(tempServerList) <= MinValidCount {
		return
	}
	ServData.ServersList = tempServerList
	ServData.ServersCount = len(tempServerList)
	WriteServerList(ServData)
}

func sortServers(list []ServerListItem) []ServerListItem {
	sort.Slice(list, func(i, j int) bool {
		iNum := len(list[i].Players)
		jNum := len(list[j].Players)
		if iNum == jNum {
			return list[i].Name < list[j].Name
		}
		return iNum > jNum
	})
	return list
}
