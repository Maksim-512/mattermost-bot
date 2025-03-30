package models

type Vote struct {
	ID        string         `json:"id"`
	Question  string         `json:"question"`
	Options   map[string]int `json:"options"`
	CreatedBy string         `json:"created_by"`
	IsClosed  bool           `json:"is_closed"`
}
