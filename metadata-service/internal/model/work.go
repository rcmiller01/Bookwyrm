package model

type Work struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	NormalizedTitle string    `json:"-"`
	FirstPubYear    int       `json:"first_pub_year,omitempty"`
	Fingerprint     string    `json:"-"`
	Authors         []Author  `json:"authors,omitempty"`
	Editions        []Edition `json:"editions,omitempty"`
	Confidence      float64   `json:"confidence,omitempty"`
}
