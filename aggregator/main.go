package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Signal struct {
	Name 	string 	`json:"name"`
	State 	string 	`json:"state"`
	Busy 	bool 	`json:"busy"`
}

func main() {
	http.HandleFunc("/status", handleStatus)
	log.Fatal(http.ListenAndServe(":8081", nil))
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
