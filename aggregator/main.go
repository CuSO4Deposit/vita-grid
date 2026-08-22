package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Config struct {
	Listen  string            `json:"listen"`
	Sources []json.RawMessage `json:"sources"`
}

type Server struct {
	sources []Source
}

type Signal struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Busy  bool   `json:"busy"`
}

type Source interface {
	Fetch() ([]Signal, error)
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
	srv := &Server{sources: sources}

	http.HandleFunc("/status", srv.handleStatus)
	log.Fatal(http.ListenAndServe(config.Listen, nil))
}

func worst(a string, b string) string {
	if a == "error" || b == "error" {
		return "error"
	}
	if a == "warn" || b == "warn" {
		return "warn"
	}
	return "ok"
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
	signals := []Signal{
		{Name: "01", State: "ok", Busy: false},
		{Name: "02", State: "warn", Busy: false},
		{Name: "03", State: "error", Busy: true},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signals)
}
