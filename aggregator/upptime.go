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
	Repo   string              `json:"repo"`
	Branch string              `json:"branch"`
	Groups map[string][]string `json:"groups"`
}

func (u *UpptimeConfig) Fetch() ([]shared.Signal, error) {
	m := upptimeStatus(u)

	keys := make([]string, 0, len(m))
	var out []shared.Signal
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, m[k])
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

func upptimeStatus(upptimeConfig *UpptimeConfig) map[string]shared.Signal {
	signals := make(map[string]shared.Signal)
	for host, sites := range upptimeConfig.Groups {
		state := shared.StateOK
		for _, site := range sites {
			githubusercontentResp, err := fetchHistoryFile(upptimeConfig.Repo, upptimeConfig.Branch, site)
			if err != nil {
				// Do not block other sites, fetch failure as warning
				state = worst(state, "warn")
				continue
			}

			upptimeResp, err := parseSingleUpptimeResponse(githubusercontentResp)
			if err != nil {
				state = worst(state, "warn")
				continue
			}

			if upptimeResp.Status != "up" {
				state = worst(state, "error")
				continue
			}
		}
		signals[host] = shared.Signal{Name: host, State: state, Busy: false}
	}
	return signals
}
