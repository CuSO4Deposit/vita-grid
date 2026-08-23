package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"vita-grid/shared"
)

type Config struct {
	Listen string        `json:"listen"`
	Probes []ProbeConfig `json:"signals"`
	Host   string        `json:"host"`
}

type ProbeConfig struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

func main() {
	configPath := flag.String("config", "config.json", "config file path")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
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
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode([]shared.Signal{
		{Name: "docker", State: shared.StateOK},
	})
}
