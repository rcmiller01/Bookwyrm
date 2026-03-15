package model

type Author struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortName  string `json:"sort_name,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}
