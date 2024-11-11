package main

import (
	"time"
)

type appVersionData struct {
	Build_mode    string
	Build_version int
	Game_version  string
	Platform      string
}

type CacheData struct {
	Version int
	Servers []ServerListItem
}

type ServerListItem struct {
	Application_version appVersionData
	Description         string
	Game_time_elapsed   interface{}
	Has_password        bool
	Host_address        string
	Mod_count           int
	Name                string
	Players             []string
	Tags                []string

	//Local data
	Local ServerMetaData
}

// Server state
type ServerStateData struct {
	URL, Query, Token, Username *string
	ServerList                  CacheData
	LastRefresh                 time.Time
	LastAttempt                 time.Time
	ServersCount,
	PlayerCount,
	NumPages,
	CurrentPage,
	ItemsPerPage int

	FTag, FName, FDesc, FPlayer    bool
	SPlayers, SName, STime, SRTime bool
	VanillaOnly, ModdedOnly        bool
	HasPass, AnyPass               bool
	HasPlay, NoPlay                bool
	VersionList                    []VersionData

	FVersion, UserAgent, Searched string
}

type VersionData struct {
	Version string
	Count   int
}

type versionInt struct {
	a, b, c int
}

type ServerMetaData struct {
	ConnectURL string
	TimeStr    string
	Minutes    int
	Modded     bool

	Icon     string
	Homepage string
	Discord  string
}
