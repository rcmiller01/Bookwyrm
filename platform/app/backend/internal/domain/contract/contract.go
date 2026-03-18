package contract

import "context"

type Domain interface {
	Name() string
	QueryBuilder() QueryBuilder
	MatchEngine() MatchEngine
	NamingEngine() NamingEngine
	ImportRules() ImportRules
}

type QueryBuilder interface {
	BuildSearch(workID string, prefs Preferences) QuerySpec
}

type MatchEngine interface {
	Match(ctx context.Context, input MatchInput) MatchResult
}

type NamingEngine interface {
	Plan(ctx context.Context, input NamingInput) (NamingPlan, error)
}

type ImportRules interface {
	SupportedExtensions() map[string]bool
	IsJunk(filename string) bool
	GroupFiles(files []string) []FileGroup
}

type Preferences struct {
	Snapshot              MetadataSnapshot
	RequestedCapabilities []string
	Priority              string
	PolicyProfile         string
	BackendGroups         []string
}

type MetadataSnapshot struct {
	WorkID          string
	EditionID       string
	ISBN10          string
	ISBN13          string
	Title           string
	Authors         []string
	Language        string
	PublicationYear int
}

type QuerySpec struct {
	Metadata              map[string]any
	RequestedCapabilities []string
	Priority              string
	PolicyProfile         string
	BackendGroups         []string
}

type MatchInput struct {
	Files []string
}

type MatchResult struct {
	Candidates []EntityRef
	Confidence float64
}

type NamingInput struct {
	Entity EntityRef
	Files  []string
}

type NamingPlan struct {
	Variables map[string]string
	Renames   map[string]string
}

type FileGroup struct {
	Key   string
	Files []string
}
