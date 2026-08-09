// Command render-readme-screenshot renders the real application template with
// deterministic demo data and captures it for the README.
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const screenshotName = "docs/server-browser.png"

type demoVersion struct {
	Game_version string
}

type demoMeta struct {
	ConnectURL       template.URL
	TimeStr          string
	Modded           bool
	SpaceAgeRequired bool
	Players          int
	HasPlayers       bool
}

type demoServer struct {
	Application_version demoVersion
	Description         string
	Has_password        bool
	Mod_count           int
	Name                string
	Tags                []string
	Local               demoMeta
}

type demoServerList struct {
	Servers []demoServer
}

type demoVersionCount struct {
	Version string
	Count   int
}

type demoPage struct {
	PlayerCount int
	ServerList  demoServerList
	VersionList []demoVersionCount
	CurrentPage int
	NumPages    int

	FTag, FName, FDesc, FPlayer    bool
	SPlayers, SName, STime, SRTime bool
	VanillaOnly, ModdedOnly        bool
	HasPass, AnyPass               bool
	HasPlay, NoPlay                bool
	SpaceAgeOnly, NoSpaceAge       bool

	FVersion string
	Searched string
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}

	pageTemplate, err := template.ParseFiles(filepath.Join(repoRoot, "data", "template.html"))
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := pageTemplate.Execute(w, screenshotData()); err != nil {
			log.Printf("render template: %v", err)
		}
	}))
	defer server.Close()

	browser, err := findBrowser()
	if err != nil {
		log.Fatal(err)
	}

	outputPath := filepath.Join(repoRoot, screenshotName)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		log.Fatalf("create screenshot directory: %v", err)
	}

	cmd := exec.Command(browser,
		"--headless=new",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--hide-scrollbars",
		"--window-size=1600,900",
		"--virtual-time-budget=1000",
		"--screenshot="+outputPath,
		server.URL,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("capture screenshot: %v\n%s", err, output)
	}

	fmt.Printf("Wrote %s\n", outputPath)
}

func findRepoRoot() (string, error) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate render script")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..")), nil
}

func findBrowser() (string, error) {
	if configured := os.Getenv("SCREENSHOT_BROWSER"); configured != "" {
		browser, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("find SCREENSHOT_BROWSER %q: %w", configured, err)
		}
		return browser, nil
	}

	for _, candidate := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if browser, err := exec.LookPath(candidate); err == nil {
			return browser, nil
		}
	}
	return "", fmt.Errorf("Chrome or Chromium is required; set SCREENSHOT_BROWSER to its executable")
}

func screenshotData() demoPage {
	return demoPage{
		PlayerCount: 184,
		CurrentPage: 2,
		NumPages:    9,
		SPlayers:    true,
		VersionList: []demoVersionCount{
			{Version: "2.0.66", Count: 143},
			{Version: "2.0.65", Count: 28},
			{Version: "1.1.110", Count: 11},
		},
		ServerList: demoServerList{Servers: []demoServer{
			{
				Name:                "M45 Science - Space Age",
				Description:         "Cooperative factory building with a friendly community and regular events.",
				Mod_count:           14,
				Tags:                []string{"m45", "space-age", "cooperative"},
				Application_version: demoVersion{Game_version: "2.0.66"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "3 hrs, 42 mins", Modded: true, SpaceAgeRequired: true, Players: 18, HasPlayers: true},
			},
			{
				Name:                "The Copper Circuit",
				Description:         "Vanilla megabase. New players welcome.",
				Tags:                []string{"vanilla", "megabase", "public"},
				Application_version: demoVersion{Game_version: "2.0.66"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "1 day, 8 hrs", Players: 12, HasPlayers: true},
			},
			{
				Name:                "Railworld Engineers",
				Description:         "Long-distance logistics and high-throughput rail networks.",
				Has_password:        true,
				Mod_count:           6,
				Tags:                []string{"rail-world", "modded", "logistics"},
				Application_version: demoVersion{Game_version: "2.0.65"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "6 hrs, 19 mins", Modded: true, Players: 9, HasPlayers: true},
			},
			{
				Name:                "Fresh Nauvis Start",
				Description:         "A relaxed public server starting from the burner phase.",
				Tags:                []string{"new-game", "casual", "vanilla"},
				Application_version: demoVersion{Game_version: "2.0.66"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "47 mins", Players: 6, HasPlayers: true},
			},
			{
				Name:                "Factory Must Grow",
				Description:         "Marathon settings, quality production, and interplanetary logistics.",
				Mod_count:           21,
				Tags:                []string{"marathon", "space-age", "quality"},
				Application_version: demoVersion{Game_version: "2.0.66"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "4 days, 2 hrs", Modded: true, SpaceAgeRequired: true, Players: 3, HasPlayers: true},
			},
			{
				Name:                "Quiet Starter Base",
				Description:         "A small vanilla factory waiting for its next engineer.",
				Tags:                []string{"vanilla", "beginner-friendly"},
				Application_version: demoVersion{Game_version: "1.1.110"},
				Local:               demoMeta{ConnectURL: "steam://connect/example", TimeStr: "2 hrs, 5 mins"},
			},
		}},
	}
}
