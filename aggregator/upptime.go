package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"vita-grid/shared"
)

type UpptimeConfig struct {
	Repo    string        `json:"repo"`
	Branch  string        `json:"branch"`
	Sites   []UpptimeSite `json:"sites"`
	Refresh int           `json:"refresh"`
}

type UpptimeSite struct {
	Name string `json:"name"`
	File string `json:"file"`
}

func (u *UpptimeConfig) Fetch() ([]shared.Signal, error) {
	var out []shared.Signal
	for _, site := range u.Sites {
		name := site.Name
		file := site.File
		if name == "" {
			name = file
		}
		if file == "" {
			out = append(out, shared.Signal{Name: name, State: shared.StateWarn, Busy: false})
			continue
		}

		body, err := fetchHistoryFile(u.Repo, u.Branch, file)
		if err != nil {
			out = append(out, shared.Signal{Name: name, State: shared.StateWarn, Busy: false})
			continue
		}

		resp, err := parseSingleUpptimeResponse(body)
		if err != nil {
			out = append(out, shared.Signal{Name: name, State: shared.StateWarn, Busy: false})
			continue
		}

		state := shared.StateOK
		if resp.Status != "up" {
			state = shared.StateError
		}
		out = append(out, shared.Signal{Name: name, State: state, Busy: false})
	}
	return out, nil
}

type UpptimeResponse struct {
	Url         string
	Status      string
	LastUpdated time.Time
}

var client = &http.Client{Timeout: 10 * time.Second}

func fetchHistoryFile(repo string, branch string, file string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/history/%s.yml", repo, branch, file)
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
