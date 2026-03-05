package model

type Work struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	NormalizedTitle    string    `json:"-"`
	FirstPubYear       int       `json:"first_pub_year,omitempty"`
	Fingerprint        string    `json:"-"`
	SeriesName         *string   `json:"series_name,omitempty"`
	SeriesIndex        *float64  `json:"series_index,omitempty"`
	Subjects           []string  `json:"subjects,omitempty"`
	RelatedProviderIDs []string  `json:"related_provider_ids,omitempty"`
	Authors            []Author  `json:"authors,omitempty"`
	Editions           []Edition `json:"editions,omitempty"`
	Confidence         float64   `json:"confidence,omitempty"`
}
