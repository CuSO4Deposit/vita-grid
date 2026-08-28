package main

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"vita-grid/shared"
)

type UpptimeConfig struct {
	Repo    string              `json:"repo"`
	Branch  string              `json:"branch"`
	Groups  map[string][]string `json:"groups"`
	Refresh int                 `json:"refresh"`
}

func (u *UpptimeConfig) Fetch() ([]shared.Signal, error) {
	keys := make([]string, 0, len(u.Groups))
	for k := range u.Groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []shared.Signal
	for _, host := range keys {
		out = append(out, upptimeGroupStatus(u, host)...)
	}
	return out, nil
}

type UpptimeResponse struct {
	Url         string
	Status      string
	LastUpdated time.Time
}

var client = &http.Client{Timeout: 10 * time.Second}

func fetchHistoryFile(repo string, branch string, siteName string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/history/%s.yml", repo, branch, siteName)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: http %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func parseSingleUpptimeResponse(respBytes []byte) (*UpptimeResponse, error) {
	resp := string(respBytes)
	lines := strings.Split(resp, "\n")

	var url string
	var status string
	var lastUpdated time.Time

	kv := make(map[string]string)
	for _, line := range lines {
		p := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(p) == 2 {
			kv[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1])
		}
	}

	url = kv["url"]
	status = kv["status"]
	// lastUpdated do not reflect true updates from upptime,
	// 	because upptime skips commit when status doesn't change
	if v, err := time.Parse(time.RFC3339, kv["lastUpdated"]); err == nil {
		lastUpdated = v
	}
	parsed := UpptimeResponse{Url: url, Status: status, LastUpdated: lastUpdated}
	return &parsed, nil
}

func slugify(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

func upptimeGroupStatus(upptimeConfig *UpptimeConfig, host string) []shared.Signal {
	state := shared.StateOK
	var sites []shared.Signal
	for _, site := range upptimeConfig.Groups[host] {
		githubusercontentResp, err := fetchHistoryFile(upptimeConfig.Repo, upptimeConfig.Branch, slugify(site))
		if err != nil {
			// Do not block other sites, fetch failure as warning
			sites = append(sites, shared.Signal{Name: host + ":" + site, State: shared.StateWarn, Busy: false})
			state = worst(state, shared.StateWarn)
			continue
		}

		upptimeResp, err := parseSingleUpptimeResponse(githubusercontentResp)
		if err != nil {
			sites = append(sites, shared.Signal{Name: host + ":" + site, State: shared.StateWarn, Busy: false})
			state = worst(state, shared.StateWarn)
			continue
		}

		siteState := shared.StateOK
		if upptimeResp.Status != "up" {
			siteState = shared.StateError
		}
		sites = append(sites, shared.Signal{Name: host + ":" + site, State: siteState, Busy: false})
		state = worst(state, siteState)
	}
	return append([]shared.Signal{{Name: host, State: state, Busy: false}}, sites...)
}
