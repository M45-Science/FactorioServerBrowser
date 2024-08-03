package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"goFactServView/cwlog"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	Version   = "0.1.8"
	VDate     = "08032024-1244p"
	ProgName  = "goFactServView"
	UserAgent = ProgName + "-" + Version
	VString   = ProgName + "v" + Version + " (" + VDate + ") "

	//How long to wait for list server
	ReqTimeout = time.Second * 5
	//How often we can make a request, including on error.
	ReqThrottle = time.Second * 15
	//How often we can refresh when new requests come in
	RefreshInterval = time.Minute * 5
	//Timeout before our http(s) servers time out
	ServerTimeout = 10 * time.Second

	//How often to refresh if there are no requests
	BGFetchInterval = time.Hour * 3

	//If we get less results than this, assume the data is incomplete or corrupt
	MinValidCount = 25

	//Servers per page
	ItemsPerPage = 25
)

var (
	sParam ServerStateData
	tmpl   *template.Template

	bindIP        *string
	bindPortHTTPS *int
	bindPortHTTP  *int

	fileServer http.Handler
)

func main() {

	//Parse parameters
	sParam = ServerStateData{UserAgent: UserAgent}
	sParam.URL = flag.String("url", "multiplayer.factorio.com", "domain name to query")
	sParam.Token = flag.String("token", "", "Matchmaking API token")
	sParam.Username = flag.String("username", "", "Matchmaking API username")

	bindIP = flag.String("ip", "", "IP to bind to")
	bindPortHTTPS = flag.Int("httpsPort", 443, "port to bind to for HTTPS")
	bindPortHTTP = flag.Int("httpPort", 80, "port to bind to")
	flag.Parse()

	//Require token/username
	if *sParam.Token == "" || *sParam.Username == "" {
		cwlog.DoLog(false, "You must supply a username and token. -h for help.")
		os.Exit(1)
		return
	}

	//Defer to give log time to write on close
	defer time.Sleep(time.Second * 2)
	cwlog.StartLog()
	cwlog.LogDaemon()

	//Pretty time formatting
	setupDurafmt()

	//Read cache.json
	ReadServerCache()

	//Parse template.html
	parseTemplate()

	//HTTP(s) fileserver
	fileServer = http.FileServer(http.Dir("data/www"))

	go backgroundUpdateList()

	//HTTP listen
	go func() {
		buf := fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTP)
		if err := http.ListenAndServe(buf, http.HandlerFunc(reqHandle)); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	http.HandleFunc("/", reqHandle)

	cert := loadCerts()
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", *bindIP, *bindPortHTTPS),
		Handler:      http.DefaultServeMux,
		TLSConfig:    config,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),

		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
		IdleTimeout:  ServerTimeout,
	}

	go autoUpdateCert()

	//https listen
	cwlog.DoLog(true, "Server started.")
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		cwlog.DoLog(true, "ListenAndServeTLS: %v", err)
		panic(err)
	}

	cwlog.DoLog(true, "Goodbye.")
}
