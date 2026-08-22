package main

type WebConfig struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func (w *WebConfig) Fetch() ([]Signal, error) {
	resp, err := client.Get(w.URL)
	if err != nil {
		return []Signal{{Name: w.Name, State: "error"}}, nil
	}
	defer resp.Body.Close()

	state := "error"
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		state = "ok"
	}
	return []Signal{{Name: w.Name, State: state}}, nil
}
