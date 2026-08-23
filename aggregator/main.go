package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"vita-grid/shared"
)

type Config struct {
	Listen  string            `json:"listen"`
	Sources []json.RawMessage `json:"sources"`
	Refresh int               `json:"refresh"`
}

type Server struct {
	sources []Source
	mutex   sync.Mutex
	cache   []shared.Signal
}

type Source interface {
	Fetch() ([]shared.Signal, error)
}

func newSource(raw json.RawMessage) (Source, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case "upptime":
		var u UpptimeConfig
		if err := json.Unmarshal(raw, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case "web":
		var w WebConfig
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, err
		}
		return &w, nil
	case "statusd":
		var sd StatusdConfig
		if err := json.Unmarshal(raw, &sd); err != nil {
			return nil, err
		}
		return &sd, nil
	default:
		return nil, fmt.Errorf("unknown source type: %q", probe.Type)
	}
}

func main() {
	configPath := flag.String("config", "config.json", "config file path")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	sources := make([]Source, 0, len(config.Sources))
	for _, raw := range config.Sources {
		s, err := newSource(raw)
		if err != nil {
			log.Fatalf("bad source: %v", err)
		}
		sources = append(sources, s)
	}
	srv := &Server{sources: sources, cache: []shared.Signal{}}

	go srv.refreshLoop(config.Refresh)

	http.HandleFunc("/status", srv.handleStatus)
	log.Fatal(http.ListenAndServe(config.Listen, nil))
}

func worst(a shared.State, b shared.State) shared.State {
	if a == shared.StateError || b == shared.StateError {
		return shared.StateError
	}
	if a == shared.StateWarn || b == shared.StateWarn {
		return shared.StateWarn
	}
	return shared.StateOK
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c Config
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mutex.Lock()
	json.NewEncoder(w).Encode(s.cache)
	s.mutex.Unlock()
}

func (s *Server) refresh() {
	var all []shared.Signal
	for _, src := range s.sources {
		sigs, err := src.Fetch()
		if err != nil {
			log.Printf("source: %v", err)
		}
		all = append(all, sigs...)
	}
	s.mutex.Lock()
	s.cache = all
	s.mutex.Unlock()
}

func (s *Server) refreshLoop(refresh int) {
	for {
		s.refresh()
		time.Sleep(time.Duration(refresh) * time.Second)
	}
}
