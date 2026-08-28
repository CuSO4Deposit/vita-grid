package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
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
	refresh []int
	cache   [][]shared.Signal
	mutex   sync.Mutex
}

type Source interface {
	Fetch() ([]shared.Signal, error)
}

func refreshOrDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func newSource(raw json.RawMessage, defaultRefresh int) (Source, int, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, 0, err
	}
	switch probe.Type {
	case "upptime":
		var u UpptimeConfig
		if err := json.Unmarshal(raw, &u); err != nil {
			return nil, 0, err
		}
		return &u, refreshOrDefault(u.Refresh, defaultRefresh), nil
	case "web":
		var w WebConfig
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, 0, err
		}
		return &w, refreshOrDefault(w.Refresh, defaultRefresh), nil
	case "statusd":
		var sd StatusdConfig
		if err := json.Unmarshal(raw, &sd); err != nil {
			return nil, 0, err
		}
		return &sd, refreshOrDefault(sd.Refresh, defaultRefresh), nil
	default:
		return nil, 0, fmt.Errorf("unknown source type: %q", probe.Type)
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
	refresh := make([]int, 0, len(config.Sources))
	for _, raw := range config.Sources {
		s, r, err := newSource(raw, config.Refresh)
		if err != nil {
			log.Fatalf("bad source: %v", err)
		}
		sources = append(sources, s)
		refresh = append(refresh, r)
	}
	srv := &Server{
		sources: sources,
		refresh: refresh,
		cache:   make([][]shared.Signal, len(sources)),
	}

	srv.start()

	http.HandleFunc("/status", srv.handleStatus)
	http.HandleFunc("/status/text", srv.handleStatusText)
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
	var all []shared.Signal
	for _, c := range s.cache {
		all = append(all, c...)
	}
	json.NewEncoder(w).Encode(all)
	s.mutex.Unlock()
}

func (s *Server) handleStatusText(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	s.mutex.Lock()
	var all []shared.Signal
	for _, c := range s.cache {
		all = append(all, c...)
	}
	s.mutex.Unlock()

	var errors, warns []string
	for _, sig := range all {
		switch sig.State {
		case shared.StateError:
			errors = append(errors, sig.Name)
		case shared.StateWarn:
			warns = append(warns, sig.Name)
		}
	}
	sort.Strings(errors)
	sort.Strings(warns)

	if len(errors) == 0 && len(warns) == 0 {
		fmt.Fprintln(w, "all ok")
		return
	}
	for _, name := range errors {
		fmt.Fprintf(w, "RED: %s\n", name)
	}
	for _, name := range warns {
		fmt.Fprintf(w, "YELLOW: %s\n", name)
	}
}

func (s *Server) start() {
	for i, src := range s.sources {
		go s.sourceLoop(i, src)
	}
}

func (s *Server) sourceLoop(i int, src Source) {
	for {
		sigs, err := src.Fetch()
		if err != nil {
			log.Printf("source %d: %v", i, err)
		}
		s.mutex.Lock()
		s.cache[i] = sigs
		s.mutex.Unlock()
		time.Sleep(time.Duration(s.refresh[i]) * time.Second)
	}
}
