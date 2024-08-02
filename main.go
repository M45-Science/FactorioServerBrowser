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
	"regexp"
	"time"

	"github.com/hako/durafmt"
)

const (
	Version      = "0.1.5"
	VDate        = "08012024-0812p"
	ProgName     = "goFactServView"
	CacheVersion = 1
	UserAgent    = ProgName + "-" + Version
	VString      = ProgName + "v" + Version + " (" + VDate + ") "
	CacheFile    = "data/cache.json"

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

	remFactTag      *regexp.Regexp = regexp.MustCompile(`\[/[^][]+\]`)
	remFactCloseTag *regexp.Regexp = regexp.MustCompile(`\[(.*?)=(.*?)\]`)
	tUnits          durafmt.Units
	fileServer      http.Handler
)

func main() {

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

	defer time.Sleep(time.Second * 2)

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
	tmpl, err = template.ParseFiles("data/template.html")
	if err != nil {
		panic(err)
	}

	/* Download server */
	fileServer = http.FileServer(http.Dir("data/www"))

	//Refresh cache infrequently
	go func() {
		for {
			time.Sleep(BGFetchInterval)
			FetchServerList()
		}
	}()

	/* Load certificates */
	cert, err := tls.LoadX509KeyPair("data/certs/fullchain.pem", "data/certs/privkey.pem")
	if err != nil {
		cwlog.DoLog(true, "Error loading TLS key pair: %v data/certs/(fullchain.pem, privkey.pem)", err)
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

	go autoUpdateCert()

	// Start server
	cwlog.DoLog(true, "Server started.")
	err = server.ListenAndServeTLS("", "")
	if err != nil {
		cwlog.DoLog(true, "ListenAndServeTLS: %v", err)
		panic(err)
	}

	cwlog.DoLog(true, "Goodbye.")
}
