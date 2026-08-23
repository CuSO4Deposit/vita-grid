package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"vita-grid/shared"
)

type Config struct {
	Listen string        `json:"listen"`
	Probes []ProbeConfig `json:"probes"`
}

type ProbeConfig struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

type Server struct {
	config *Config
}

func main() {
	configPath := flag.String("config", "config.json", "config file path")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	srv := Server{config: config}

	http.HandleFunc("/status", srv.handleStatus)
	log.Fatal(http.ListenAndServe(config.Listen, nil))
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

	json.NewEncoder(w).Encode(
		collectSignals(s.config),
	)
}

func collectSignals(config *Config) []shared.Signal {
	var out []shared.Signal
	for _, probe := range config.Probes {
		var state shared.State
		switch probe.Kind {
		case "systemd-active":
			state = checkSystemdActive(probe.Target)
		default:
			state = shared.StateWarn
		}
		out = append(out, shared.Signal{Name: probe.Name, State: state})
	}
	return out
}

func checkSystemdActive(unit string) shared.State {
	out, err := exec.Command("systemctl", "is-active", unit).Output()
	if err != nil {
		return shared.StateError
	}
	if strings.TrimSpace(string(out)) == "active" {
		return shared.StateOK
	}
	return shared.StateWarn
}
