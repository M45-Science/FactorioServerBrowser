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

type modDescriptionData struct {
	Name    string
	Version string
}

type ServerListItem struct {
	Application_version       appVersionData
	Description               string
	Game_id                   int
	Game_time_elapsed         interface{}
	Has_password              bool
	Host_address              string
	Max_players               int
	Mod_count                 int
	Modded                    bool
	Mods                      []modDescriptionData
	Name                      string
	Players                   []string
	Require_user_verification bool
	Server_id                 string
	Tags                      []string

	ConnectURL string
	Position   int
	Time       string
}

type ServerStateData struct {
	URL, Query, Token, Username *string
	ServersList                 []ServerListItem
	LastRefresh                 time.Time
	LastAttempt                 time.Time
	ServersCount,
	NumPages,
	CurrentPage,
	ItemsPerPage int

	UserAgent, Version string
}
