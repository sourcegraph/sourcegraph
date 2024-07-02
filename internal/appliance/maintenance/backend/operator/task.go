package operator

import "time"

type Task struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Started     bool      `json:"started"`
	Finished    bool      `json:"finished"`
	Weight      int       `json:"weight"`
	Progress    int       `json:"progress"`
	LastUpdate  time.Time `json:"lastUpdate"`
}
