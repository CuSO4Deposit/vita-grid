package shared

type State string

const (
	StateOK    State = "ok"
	StateWarn  State = "warn"
	StateError State = "error"
)

type Signal struct {
	Name  string `json:"name"`
	State State  `json:"state"`
	Busy  bool   `json:"busy"`
}
