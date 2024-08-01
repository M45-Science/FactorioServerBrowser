package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"goFactServView/cwlog"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hako/durafmt"
)

const (
	Version   = "0.1.5"
	VDate     = "07312024-0958p"
	ProgName  = "goFactServView"
	UserAgent = ProgName + "-" + Version
	VString   = ProgName + "v" + Version + " (" + VDate + ") "
	CacheFile = "cache.json"

	ReqTimeout      = time.Second * 5
	ReqThrottle     = time.Second * 15
	RefreshInterval = time.Minute * 5
	ServerTimeout   = 10 * time.Second

	BGFetchInterval = time.Hour * 3
	//If we get less results than this, assume the data is incomplete or corrupt
	MinValidCount = 25
	ItemsPerPage  = 25
)

var (
	sParam ServerStateData
	tmpl   *template.Template

	bindIP        *string
	bindPortHTTPS *int
	bindPortHTTP  *int

	rega       *regexp.Regexp = regexp.MustCompile(`\[/[^][]+\]`)
	regb       *regexp.Regexp = regexp.MustCompile(`\[(.*?)=(.*?)\]`)
	tUnits     durafmt.Units
	fileServer http.Handler
)

func main() {
	defer time.Sleep(time.Second * 2)
	sParam = ServerStateData{UserAgent: UserAgent}
	sParam.URL = flag.String("url", "multiplayer.factorio.com", "domain name to query")
	sParam.Token = flag.String("token", "", "Matchmaking API token")
	sParam.Username = flag.String("username", "", "Matchmaking API username")

	bindIP = flag.String("ip", "", "IP to bind to")
	bindPortHTTPS = flag.Int("httpsPort", 443, "port to bind to for HTTPS")
	bindPortHTTP = flag.Int("httpPort", 80, "port to bind to")

	flag.Parse()

	if *sParam.Token == "" || *sParam.Username == "" {
		cwlog.DoLog(false, "You must supply a username and token. -h for help.")
		os.Exit(1)
		return
	}

	var err error
	tUnits, err = durafmt.DefaultUnitsCoder.Decode("yr:yrs,wk:wks,day:days,hr:hrs,min:mins,sec:secs,ms:ms,μs:μs")
	if err != nil {
		panic(err)
	}

	cwlog.StartLog()
	cwlog.LogDaemon()

	go func() {
		buf := fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTP)
		if err := http.ListenAndServe(buf, http.HandlerFunc(httpsHandler)); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	//Read server cache
	ReadServerList()

	//Parse template
	tmpl, err = template.ParseFiles("template.html")
	if err != nil {
		panic(err)
	}

	/* Download server */
	fileServer = http.FileServer(http.Dir("www"))

	//Refresh cache infrequently
	go func() {
		for {
			time.Sleep(BGFetchInterval)
			FetchServerList()
		}
	}()

	/* Load certificates */
	cert, err := tls.LoadX509KeyPair("fullchain.pem", "privkey.pem")
	if err != nil {
		cwlog.DoLog(true, "Error loading TLS key pair: %v (fullchain.pem, privkey.pem)", err)
		return
	}
	cwlog.DoLog(true, "Loaded certs.")

	/* HTTPS server */
	http.HandleFunc("/", httpsHandler)

	/* Create TLS configuration */
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	/* Create HTTPS server */
	server := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTPS),
		Handler:      http.DefaultServeMux,
		TLSConfig:    config,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),

		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
		IdleTimeout:  ServerTimeout,
	}

	go func() {
		for {
			time.Sleep(time.Minute)

			filePath := "fullchain.pem"
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
	}()

	// Start server
	cwlog.DoLog(true, "Starting server...")
	err = server.ListenAndServeTLS("", "")
	if err != nil {
		cwlog.DoLog(true, "ListenAndServeTLS: %v", err)
		panic(err)
	}

	cwlog.DoLog(true, "Goodbye.")
}

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
		ServersList:  sParam.ServersList,
		ServersCount: sParam.ServersCount,
		ItemsPerPage: ItemsPerPage,
	}

	tempServersList := []ServerListItem{}
	page := 1

	cwlog.DoLog(false, "Request: %v", r.RequestURI)

	queryItems := r.URL.Query()
	if len(queryItems) > 0 {
		//cwlog.DoLog(false, "Query: %v", queryItems)
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
			} else if !found && strings.EqualFold(key, "name") {
				for s, server := range tempParams.ServersList {
					lName := strings.ToLower(server.Name)
					lVal := strings.ToLower(values[0])
					if strings.Contains(lName, lVal) {
						tempServersList = append(tempServersList, tempParams.ServersList[s])
					}
				}
				found = true
			} else if !found && strings.EqualFold(key, "desc") {
				for s, server := range tempParams.ServersList {
					lDesc := strings.ToLower(server.Description)
					lVal := strings.ToLower(values[0])
					if strings.Contains(lDesc, lVal) {
						tempServersList = append(tempServersList, tempParams.ServersList[s])
					}
				}
				found = true
			} else if !found && strings.EqualFold(key, "tag") {
				for s, server := range tempParams.ServersList {
					for _, tag := range server.Tags {
						if strings.EqualFold(values[0], tag) {
							tempServersList = append(tempServersList, tempParams.ServersList[s])
							break
						}
					}
				}
				found = true
			} else if !found && strings.EqualFold(key, "player") {
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
	tempParams.NumPages = int(math.Ceil(float64(tempParams.ServersCount) / float64(tempParams.ItemsPerPage)))
	if page > tempParams.NumPages {
		tempParams.CurrentPage = tempParams.NumPages
	} else if page < 1 {
		tempParams.CurrentPage = 0
	} else {
		tempParams.CurrentPage = page
	}
}

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
		tempServerList[i].Time = updateTime(item)
	}
	tempServerList = sortServers(tempServerList)

	sParam.LastRefresh = time.Now().UTC()
	cwlog.DoLog(false, "Fetched server list at %v", time.Now())

	if len(tempServerList) <= MinValidCount {
		return
	}
	sParam.ServersList = tempServerList
	sParam.ServersCount = len(tempServerList)
	WriteServerList()
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
