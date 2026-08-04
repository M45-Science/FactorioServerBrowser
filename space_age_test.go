package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRequiresSpaceAge(t *testing.T) {
	tests := []struct {
		name string
		mods []modInfo
		want bool
	}{
		{name: "base only", mods: []modInfo{{Name: "base"}}},
		{name: "unrelated mod", mods: []modInfo{{Name: "Krastorio2"}}},
		{name: "space age", mods: []modInfo{{Name: "space-age"}}, want: true},
		{name: "quality", mods: []modInfo{{Name: "quality"}}, want: true},
		{name: "elevated rails", mods: []modInfo{{Name: "elevated-rails"}}, want: true},
		{name: "recycler", mods: []modInfo{{Name: "recycler"}}, want: true},
		{name: "case insensitive", mods: []modInfo{{Name: "SPACE-AGE"}}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := requiresSpaceAge(test.mods); got != test.want {
				t.Fatalf("requiresSpaceAge() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPopulateSpaceAgeRequirements(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()

		switch r.URL.Path {
		case "/get-game-details/1":
			fmt.Fprint(w, `{"mods":[{"name":"quality","version":"2.0.0"}]}`)
		case "/get-game-details/2":
			fmt.Fprint(w, `{"mods":[{"name":"base","version":"2.0.0"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldClient := fetchHTTPClient
	fetchHTTPClient = func() *http.Client { return server.Client() }
	spaceAgeCache.Lock()
	oldCache := spaceAgeCache.servers
	spaceAgeCache.servers = make(map[uint64]bool)
	spaceAgeCache.Unlock()
	defer func() {
		fetchHTTPClient = oldClient
		spaceAgeCache.Lock()
		spaceAgeCache.servers = oldCache
		spaceAgeCache.Unlock()
	}()

	servers := []ServerListItem{
		{Game_id: 1, Application_version: appVersionData{Game_version: "2.0.0"}},
		{Game_id: 2, Application_version: appVersionData{Game_version: "2.0.0"}},
		{Game_id: 3, Application_version: appVersionData{Game_version: "1.1.110"}},
	}
	populateSpaceAgeRequirements(servers, server.URL)

	if !servers[0].Local.SpaceAgeRequired {
		t.Fatal("expected quality mod to require Space Age")
	}
	if servers[1].Local.SpaceAgeRequired {
		t.Fatal("expected base-only server not to require Space Age")
	}
	if servers[2].Local.SpaceAgeRequired {
		t.Fatal("expected pre-2.0 server not to require Space Age")
	}

	// A second page render should use cached results instead of repeating API calls.
	populateSpaceAgeRequirements(servers, server.URL)
	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Fatalf("expected 2 detail requests, got %d", requests)
	}
}

func TestTemplateShowsSpaceAgeRequirement(t *testing.T) {
	tmpl, err := template.ParseFiles("data/template.html")
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	params := ServerStateData{
		ServerList: CacheData{Servers: []ServerListItem{{
			Name: "Space Age server",
			Local: ServerMetaData{
				SpaceAgeRequired: true,
			},
		}}},
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, params); err != nil {
		t.Fatalf("execute template: %v", err)
	}
	if !strings.Contains(output.String(), "Space Age required") {
		t.Fatal("expected template to show the Space Age requirement")
	}
}
