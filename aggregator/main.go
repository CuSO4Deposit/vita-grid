package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
)

type Config struct {
	Listen  string            `json:"listen"`
	Sources []json.RawMessage `json:"sources"`
}

type Signal struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Busy  bool   `json:"busy"`
}

func main() {
	configPath := flag.String("config", "config.json", "config file path")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Load config failed: %v", err)
	}

	http.HandleFunc("/status", handleStatus)
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

func handleStatus(w http.ResponseWriter, _ *http.Request) {
	signals := []Signal{
		{Name: "01", State: "ok", Busy: false},
		{Name: "02", State: "warn", Busy: false},
		{Name: "03", State: "error", Busy: true},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signals)
}
