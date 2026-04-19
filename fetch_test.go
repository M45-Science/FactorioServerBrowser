package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchServerListSuccessUpdatesState(t *testing.T) {
	setupDurafmt()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get-games" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("User-Agent"); got != UserAgent {
			t.Fatalf("unexpected user agent: %s", got)
		}
		fmt.Fprint(w, makeServerListJSON(MinValidCount+1))
	}))
	defer server.Close()

	restore := configureFetchTestState(t)
	defer restore()

	*sParam.URL = server.URL
	sParam.LastRefresh = time.Time{}
	sParam.LastAttempt = time.Time{}
	sParam.ServerList.Servers = seedServers(2)
	sParam.ServersCount = len(sParam.ServerList.Servers)
	sParam.PlayerCount = 99
	fetchHTTPClient = func() *http.Client {
		return server.Client()
	}

	if err := fetchServerList(); err != nil {
		t.Fatalf("fetchServerList returned error: %v", err)
	}

	if len(sParam.ServerList.Servers) != MinValidCount+1 {
		t.Fatalf("expected %d servers, got %d", MinValidCount+1, len(sParam.ServerList.Servers))
	}
	if sParam.ServersCount != MinValidCount+1 {
		t.Fatalf("expected ServersCount to be updated, got %d", sParam.ServersCount)
	}
	if sParam.PlayerCount != MinValidCount+1 {
		t.Fatalf("expected PlayerCount to be updated, got %d", sParam.PlayerCount)
	}
	if sParam.LastRefresh.IsZero() {
		t.Fatal("expected LastRefresh to be updated")
	}
	for _, server := range sParam.ServerList.Servers {
		if strings.Contains(server.Tags[0], "[") || strings.Contains(server.Tags[0], "]") {
			t.Fatalf("expected sanitized tags, got %q", server.Tags[0])
		}
	}
}

func TestFetchServerListNetworkErrorKeepsState(t *testing.T) {
	setupDurafmt()

	restore := configureFetchTestState(t)
	defer restore()

	original := seedServers(3)
	sParam.ServerList.Servers = original
	sParam.ServersCount = len(original)
	sParam.PlayerCount = 7
	sParam.LastRefresh = time.Unix(123, 0).UTC()
	sParam.LastAttempt = time.Time{}
	fetchHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
		}
	}

	err := fetchServerList()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(sParam.ServerList.Servers) != len(original) {
		t.Fatalf("expected stale servers to remain, got %d", len(sParam.ServerList.Servers))
	}
	if sParam.PlayerCount != 7 {
		t.Fatalf("expected PlayerCount to remain unchanged, got %d", sParam.PlayerCount)
	}
	if !sParam.LastRefresh.Equal(time.Unix(123, 0).UTC()) {
		t.Fatalf("expected LastRefresh to remain unchanged, got %v", sParam.LastRefresh)
	}
}

func TestFetchServerListRejectsNon200(t *testing.T) {
	setupDurafmt()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusTooManyRequests)
	}))
	defer server.Close()

	restore := configureFetchTestState(t)
	defer restore()

	*sParam.URL = server.URL
	sParam.ServerList.Servers = seedServers(4)
	sParam.ServersCount = 4
	sParam.PlayerCount = 11
	sParam.LastRefresh = time.Unix(222, 0).UTC()
	fetchHTTPClient = func() *http.Client {
		return server.Client()
	}

	err := fetchServerList()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unexpected upstream status 429") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sParam.ServerList.Servers) != 4 {
		t.Fatalf("expected stale servers to remain, got %d", len(sParam.ServerList.Servers))
	}
	if !sParam.LastRefresh.Equal(time.Unix(222, 0).UTC()) {
		t.Fatalf("expected LastRefresh to remain unchanged, got %v", sParam.LastRefresh)
	}
}

func TestFetchServerListRejectsInvalidJSON(t *testing.T) {
	setupDurafmt()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{not-json")
	}))
	defer server.Close()

	restore := configureFetchTestState(t)
	defer restore()

	*sParam.URL = server.URL
	sParam.ServerList.Servers = seedServers(5)
	sParam.ServersCount = 5
	sParam.PlayerCount = 12
	sParam.LastRefresh = time.Unix(333, 0).UTC()
	fetchHTTPClient = func() *http.Client {
		return server.Client()
	}

	err := fetchServerList()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(sParam.ServerList.Servers) != 5 {
		t.Fatalf("expected stale servers to remain, got %d", len(sParam.ServerList.Servers))
	}
	if !sParam.LastRefresh.Equal(time.Unix(333, 0).UTC()) {
		t.Fatalf("expected LastRefresh to remain unchanged, got %v", sParam.LastRefresh)
	}
}

func TestFetchServerListRejectsUndersizedPayload(t *testing.T) {
	setupDurafmt()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, makeServerListJSON(MinValidCount))
	}))
	defer server.Close()

	restore := configureFetchTestState(t)
	defer restore()

	*sParam.URL = server.URL
	sParam.ServerList.Servers = seedServers(6)
	sParam.ServersCount = 6
	sParam.PlayerCount = 13
	sParam.LastRefresh = time.Unix(444, 0).UTC()
	fetchHTTPClient = func() *http.Client {
		return server.Client()
	}

	err := fetchServerList()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "returned only") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sParam.ServerList.Servers) != 6 {
		t.Fatalf("expected stale servers to remain, got %d", len(sParam.ServerList.Servers))
	}
	if !sParam.LastRefresh.Equal(time.Unix(444, 0).UTC()) {
		t.Fatalf("expected LastRefresh to remain unchanged, got %v", sParam.LastRefresh)
	}
}

func configureFetchTestState(t *testing.T) func() {
	t.Helper()

	oldState := sParam
	oldClient := fetchHTTPClient

	username := "user"
	token := "token"
	baseURL := "https://example.invalid"
	sParam = ServerStateData{
		URL:       &baseURL,
		Username:  &username,
		Token:     &token,
		UserAgent: UserAgent,
	}

	return func() {
		sParam = oldState
		fetchHTTPClient = oldClient
	}
}

func seedServers(count int) []ServerListItem {
	servers := make([]ServerListItem, 0, count)
	for i := 1; i <= count; i++ {
		servers = append(servers, ServerListItem{
			Name:    fmt.Sprintf("seed-%d", i),
			Players: []string{"seed-player"},
			Tags:    []string{"seed-tag"},
		})
	}
	return servers
}

func makeServerListJSON(count int) string {
	parts := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		parts = append(parts, fmt.Sprintf(`{
			"Application_version":{"Game_version":"1.1.%d","Build_mode":"","Build_version":0,"Platform":"linux"},
			"Description":"[font=default-bold]Desc %d[/font]",
			"Game_time_elapsed":%d,
			"Has_password":false,
			"Host_address":"127.0.0.1:%d",
			"Mod_count":0,
			"Name":"[color=red]Server %d[/color]",
			"Players":["player-%d"],
			"Tags":["[color=blue]tag-%d[/color]"]
		}`, i, i, i, 3000+i, i, i, i))
	}
	return "[" + strings.Join(parts, ",") + "]"
}
