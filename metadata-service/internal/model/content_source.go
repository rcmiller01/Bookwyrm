package model

import "time"

type ContentSource struct {
	ID           int64     `json:"id"`
	EditionID    string    `json:"edition_id"`
	Provider     string    `json:"provider"`
	SourceType   string    `json:"source_type"`
	SourceName   string    `json:"source_name,omitempty"`
	SourceURL    string    `json:"source_url,omitempty"`
	Availability string    `json:"availability,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FileMetadata struct {
	ID              int64     `json:"id"`
	ContentSourceID int64     `json:"content_source_id"`
	FileName        string    `json:"file_name,omitempty"`
	FileFormat      string    `json:"file_format,omitempty"`
	FileSizeBytes   int64     `json:"file_size_bytes,omitempty"`
	Language        string    `json:"language,omitempty"`
	Checksum        string    `json:"checksum,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
