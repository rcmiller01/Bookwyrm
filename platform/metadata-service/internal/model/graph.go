package model

import "time"

// Series is a canonical series node in the metadata graph.
type Series struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SeriesEntry links a work into a series in optional numeric order.
type SeriesEntry struct {
	SeriesID    string    `json:"series_id"`
	WorkID      string    `json:"work_id"`
	SeriesIndex *float64  `json:"series_index,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Subject is a canonical subject/tag node in the metadata graph.
type Subject struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorkRelationship is a directional graph edge between works.
type WorkRelationship struct {
	SourceWorkID     string    `json:"source_work_id"`
	TargetWorkID     string    `json:"target_work_id"`
	RelationshipType string    `json:"relationship_type"`
	Confidence       float64   `json:"confidence"`
	Provider         *string   `json:"provider,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
