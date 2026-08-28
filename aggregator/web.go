package main

import (
	"vita-grid/shared"
)

type WebConfig struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Refresh int    `json:"refresh"`
}

func (w *WebConfig) Fetch() ([]shared.Signal, error) {
	resp, err := client.Get(w.URL)
	if err != nil {
		return []shared.Signal{{Name: w.Name, State: "error"}}, nil
	}
	defer resp.Body.Close()

	state := shared.StateError
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		state = shared.StateOK
	}
	return []shared.Signal{{Name: w.Name, State: state}}, nil
}
