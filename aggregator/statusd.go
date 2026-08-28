package main

import (
	"encoding/json"
	"net/http"
	"vita-grid/shared"
)

type StatusdConfig struct {
	URL     string `json:"url"`
	Host    string `json:"host"`
	Refresh int    `json:"refresh"`
	fails   int
}

func (s *StatusdConfig) Fetch() ([]shared.Signal, error) {
	resp, err := client.Get(s.URL + "/status")
	if err != nil {
		return s.degrade(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.degrade(), nil
	}

	var sigs []shared.Signal
	if err := json.NewDecoder(resp.Body).Decode(&sigs); err != nil {
		return s.degrade(), nil
	}

	s.fails = 0
	out := []shared.Signal{{Name: s.Host, State: shared.StateOK}}
	for _, sig := range sigs {
		sig.Name = s.Host + ":" + sig.Name
		out = append(out, sig)
	}
	return out, nil
}

func (s *StatusdConfig) degrade() []shared.Signal {
	s.fails++
	state := shared.StateWarn
	if s.fails >= 2 {
		state = shared.StateError
	}
	return []shared.Signal{{Name: s.Host, State: state}}
}
