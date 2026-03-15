package model

type Edition struct {
	ID              string       `json:"id"`
	WorkID          string       `json:"work_id,omitempty"`
	Title           string       `json:"title,omitempty"`
	Format          string       `json:"format,omitempty"`
	Publisher       string       `json:"publisher,omitempty"`
	PublicationYear int          `json:"publication_year,omitempty"`
	Identifiers     []Identifier `json:"identifiers,omitempty"`
}
